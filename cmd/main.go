// @title           Slurmtack API
// @version         1.0
// @description     Slurmtack orchestrates node ownership switches between Slurm and OpenStack.
// @host            localhost:8080
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     JWT bearer token. Obtain via POST /v1/auth/login.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/slurmtack/slurmtack/internal/api"
	"github.com/slurmtack/slurmtack/internal/config"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/mq"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/orchestrator"
	"github.com/slurmtack/slurmtack/internal/remote"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

type mqConnection interface {
	Connect(context.Context) error
	Reconnect(context.Context) error
	Close() error
}

type mqConsumer interface {
	Run(context.Context) error
}

type mqStartupDeps struct {
	newConnection   func(string, *slog.Logger) mqConnection
	declareTopology func(mqConnection) error
	newConsumer     func(mqConnection, store.Store, *slog.Logger) mqConsumer
}

var defaultMQStartupDeps = mqStartupDeps{
	newConnection: func(url string, logger *slog.Logger) mqConnection {
		return mq.NewConnection(url, logger)
	},
	declareTopology: func(conn mqConnection) error {
		amqpConn, ok := conn.(*mq.Connection)
		if !ok {
			return fmt.Errorf("unexpected mq connection type %T", conn)
		}
		return mq.DeclareTopology(amqpConn)
	},
	newConsumer: func(conn mqConnection, s store.Store, logger *slog.Logger) mqConsumer {
		amqpConn, ok := conn.(*mq.Connection)
		if !ok {
			return mqErrorConsumer{err: fmt.Errorf("unexpected mq connection type %T", conn)}
		}
		return mq.NewConsumer(amqpConn, s, logger)
	},
}

type mqErrorConsumer struct {
	err error
}

func (c mqErrorConsumer) Run(context.Context) error {
	return c.err
}

func main() {
	baseLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(baseLogger)

	cfg, err := config.Load()
	if err != nil {
		exitWithError(baseLogger, "config.load_failed", err)
	}

	sqlStore, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		exitWithError(baseLogger, "store.init_failed", err)
	}
	defer sqlStore.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	var slurmClient *slurm.RestClient
	if cfg.SlurmAPIURL != "" {
		opts := []slurm.Option{
			slurm.WithLogger(baseLogger),
			slurm.WithSlurmUser(cfg.SlurmAPIUser),
			slurm.WithAMQPURL(cfg.AMQPURL),
			slurm.WithPlaceholderSIFPath(cfg.PlaceholderSIFPath),
		}
		if cfg.SlurmAdminTokenRenewalEnabled() {
			provider := slurm.NewSSHAdminTokenProvider(slurm.SSHAdminTokenProviderConfig{
				Runner:         buildSSHRunner(cfg, baseLogger),
				Store:          sqlStore,
				AdminUser:      cfg.SlurmAdminUser,
				LoginNode:      cfg.SSHLoginNode,
				Lifespan:       cfg.SlurmAdminTokenLifespan,
				BootstrapToken: cfg.SlurmAdminJWTToken,
				Logger:         baseLogger,
			})
			opts = append(opts, slurm.WithAdminTokenProvider(cfg.SlurmAdminUser, provider))
		} else {
			opts = append(opts, slurm.WithAdminCredentials(cfg.SlurmAdminUser, cfg.SlurmAdminJWTToken))
		}
		slurmClient = slurm.NewRestClient(cfg.SlurmAPIURL, cfg.SlurmJWTToken, opts...)
	}

	var osClient openstack.Client
	if cfg.OSAuthURL != "" {
		opts := openstack.AuthOpts{
			AuthURL:           cfg.OSAuthURL,
			Username:          cfg.OSUsername,
			Password:          cfg.OSPassword,
			ProjectName:       cfg.OSProjectName,
			UserDomainName:    "Default",
			ProjectDomainName: "Default",
		}
		var err error
		osClient, err = openstack.NewGophecloudClient(ctx, opts)
		if err != nil {
			exitWithError(baseLogger, "openstack.client_init_failed", err)
		}
	}

	// Start orchestrator
	runner := engine.NewRunner(sqlStore, baseLogger)
	sshRunner := buildSSHRunner(cfg, baseLogger)
	orch := orchestrator.New(sqlStore, runner, sshRunner, slurmClient, osClient, orchestrator.Config{
		TickInterval:            2 * time.Second,
		SSHPollInterval:         cfg.SSHPollInterval,
		SSHPollTimeout:          cfg.SSHPollTimeout,
		PlaceholderSIFPath:      cfg.PlaceholderSIFPath,
		PlaceholderSIFFile:      cfg.PlaceholderSIFFile,
		GPUPassthroughScriptDir: cfg.GPUPassthroughScriptDir,
		RemoteStagingDir:        cfg.RemoteStagingDir,
	}, baseLogger)

	// Start MQ consumer supervision.
	mqConn := startMQ(ctx, &wg, cfg.AMQPURL, sqlStore, baseLogger, defaultMQStartupDeps.withIntakeHandler(orch))

	var publisher service.EventPublisher
	if amqpConn, ok := mqConn.(*mq.Connection); ok {
		publisher = mq.NewPublisher(amqpConn, baseLogger)
	}

	svc := service.NewSwitchService(sqlStore, baseLogger, publisher).WithExecutionIntake(orch)
	if cfg.SlurmAPIURL != "" {
		svc = svc.WithSlurmWorkloadDefaults(cfg.SlurmAPIUser, cfg.SlurmJWTToken)
	}
	svc = svc.WithPlaceholderSIFDefaults(cfg.PlaceholderSIFPath, cfg.PlaceholderSIFFile)
	if cfg.SlurmCloudPartition != "" {
		svc = svc.WithSlurmCloudPartition(cfg.SlurmCloudPartition)
		if slurmClient != nil {
			svc = svc.WithSlurmPartitionLister(slurmClient)
		}
	}
	if slurmClient != nil {
		svc = svc.WithSlurmNodeStateReader(slurmClient)
	}
	var inventoryHandler *api.InventoryHandler
	if slurmClient != nil && osClient != nil {
		inventoryHandler = api.NewInventoryHandler(slurmClient, osClient, sqlStore, cfg.SlurmCloudPartition)
	}

	jwtMgr := api.NewJWTManager(cfg.JWTSigningKey, time.Hour)
	var serverOpts []api.ServerOption
	serverOpts = append(serverOpts, api.WithJWTAuth(jwtMgr, slurmClient))
	srv := api.NewServer(cfg.ListenAddr, sqlStore, svc, inventoryHandler, baseLogger, serverOpts...)

	wg.Add(1)
	go func() {
		defer wg.Done()
		baseLogger.Info("orchestrator.started")
		orch.Run(ctx)
		baseLogger.Info("orchestrator.stopped")
	}()

	// Start HTTP server
	go func() {
		baseLogger.Info("server.listening", "address", cfg.ListenAddr)
		if err := srv.Start(); err != nil {
			exitWithError(baseLogger, "server.start_failed", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	baseLogger.Info("shutdown.started")

	if err := srv.Shutdown(); err != nil {
		baseLogger.Warn("server.shutdown_failed", "error", err)
	}

	cancel()

	if mqConn != nil {
		mqConn.Close()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		baseLogger.Warn("shutdown.timeout_waiting_for_goroutines", "timeout", 30*time.Second)
	}
}

func buildSSHRunner(cfg *config.Config, logger *slog.Logger) remote.Runner {
	if cfg == nil || !cfg.SSHRunnerEnabled() {
		return nil
	}

	executor := remote.NewExecSSHExecutor(buildSSHExecutorConfig(cfg), logger)
	return remote.NewSSHRunner(executor)
}

func buildSSHExecutorConfig(cfg *config.Config) remote.SSHExecutorConfig {
	return remote.SSHExecutorConfig{
		User:         cfg.SSHUser,
		Port:         cfg.SSHPort,
		Options:      strings.Fields(cfg.SSHOptions),
		IdentityFile: cfg.SSHPrivateKeyPath,
	}
}

func startMQ(ctx context.Context, wg *sync.WaitGroup, amqpURL string, s store.Store, logger *slog.Logger, deps mqStartupDeps) mqConnection {
	deps = deps.withDefaults()
	if amqpURL == "" {
		logger.Info("mq.disabled")
		return nil
	}

	conn := deps.newConnection(amqpURL, logger)
	wg.Add(1)
	go func() {
		defer wg.Done()

		activationRetries := 0
		needConnect := true

		for {
			if needConnect {
				if err := conn.Connect(ctx); err != nil {
					return
				}
				needConnect = false
			}

			if err := deps.declareTopology(conn); err != nil {
				activationRetries++
				logger.Warn("mq.startup_activation_failed", "attempt", activationRetries, "error", err)
				if err := conn.Reconnect(ctx); err != nil {
					return
				}
				continue
			}

			if activationRetries > 0 {
				logger.Info("mq.activated_after_retry", "attempts", activationRetries)
			} else {
				logger.Info("mq.activated")
			}

			consumer := deps.newConsumer(conn, s, logger)
			logger.Info("mq.consumer.started")
			err := consumer.Run(ctx)
			if err == nil || errors.Is(err, context.Canceled) || ctx.Err() != nil {
				logger.Info("mq.consumer.stopped")
				return
			}

			logger.Warn("mq.consumer.exited", "error", err)
			logger.Info("mq.consumer.stopped")
			activationRetries++
			if err := conn.Reconnect(ctx); err != nil {
				return
			}
		}
	}()

	return conn
}

func (d mqStartupDeps) withDefaults() mqStartupDeps {
	if d.newConnection == nil {
		d.newConnection = defaultMQStartupDeps.newConnection
	}
	if d.declareTopology == nil {
		d.declareTopology = defaultMQStartupDeps.declareTopology
	}
	if d.newConsumer == nil {
		d.newConsumer = defaultMQStartupDeps.newConsumer
	}
	return d
}

func (d mqStartupDeps) withIntakeHandler(handler mq.IntakeHandler) mqStartupDeps {
	d = d.withDefaults()
	d.newConsumer = func(conn mqConnection, s store.Store, logger *slog.Logger) mqConsumer {
		amqpConn, ok := conn.(*mq.Connection)
		if !ok {
			return mqErrorConsumer{err: fmt.Errorf("unexpected mq connection type %T", conn)}
		}
		return mq.NewConsumer(amqpConn, s, logger, handler)
	}
	return d
}

func exitWithError(logger *slog.Logger, msg string, err error) {
	logger.Error(msg, "error", err)
	os.Exit(1)
}
