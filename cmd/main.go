package main

import (
	"context"
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

	svc := service.NewSwitchService(sqlStore, baseLogger)
	srv := api.NewServer(cfg.ListenAddr, cfg.APIToken, sqlStore, svc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	var slurmClient *slurm.RestClient
	if cfg.SlurmAPIURL != "" {
		slurmClient = slurm.NewRestClient(
			cfg.SlurmAPIURL,
			cfg.SlurmJWTToken,
			slurm.WithSlurmUser(cfg.SlurmAPIUser),
			slurm.WithAdminCredentials(cfg.SlurmAdminUser, cfg.SlurmAdminJWTToken),
			slurm.WithAMQPURL(cfg.AMQPURL),
			slurm.WithPlaceholderSIFPath(cfg.PlaceholderSIFPath),
		)
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
		TickInterval:    2 * time.Second,
		SSHPollInterval: cfg.SSHPollInterval,
		SSHPollTimeout:  cfg.SSHPollTimeout,
	}, baseLogger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		baseLogger.Info("orchestrator.started")
		orch.Run(ctx)
		baseLogger.Info("orchestrator.stopped")
	}()

	// Start MQ consumer if AMQP_URL is configured
	var mqConn *mq.Connection
	if cfg.AMQPURL != "" {
		mqConn = mq.NewConnection(cfg.AMQPURL, baseLogger)
		if err := mqConn.Connect(ctx); err != nil {
			baseLogger.Warn("mq.connect_failed", "error", err, "continuing_without_mq", true)
			mqConn = nil
		} else {
			if err := mq.DeclareTopology(mqConn); err != nil {
				exitWithError(baseLogger, "mq.topology_declaration_failed", err)
			}
			consumer := mq.NewConsumer(mqConn, sqlStore, baseLogger)
			wg.Add(1)
			go func() {
				defer wg.Done()
				baseLogger.Info("mq.consumer.started")
				if err := consumer.Run(ctx); err != nil {
					baseLogger.Warn("mq.consumer.exited", "error", err)
				}
				baseLogger.Info("mq.consumer.stopped")
			}()
		}
	}

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

func exitWithError(logger *slog.Logger, msg string, err error) {
	logger.Error(msg, "error", err)
	os.Exit(1)
}
