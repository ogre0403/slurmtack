package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/slurmtack/slurmtack/internal/api"
	"github.com/slurmtack/slurmtack/internal/config"
	"github.com/slurmtack/slurmtack/internal/engine"
	"github.com/slurmtack/slurmtack/internal/mq"
	"github.com/slurmtack/slurmtack/internal/openstack"
	"github.com/slurmtack/slurmtack/internal/orchestrator"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/slurm"
	"github.com/slurmtack/slurmtack/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	baseLogger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	sqlStore, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
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
			log.Fatalf("openstack client: %v", err)
		}
	}

	// Start orchestrator
	runner := engine.NewRunner(sqlStore, baseLogger)
	orch := orchestrator.New(sqlStore, runner, nil, slurmClient, osClient, orchestrator.Config{
		TickInterval:    2 * time.Second,
		SSHPollInterval: cfg.SSHPollInterval,
		SSHPollTimeout:  cfg.SSHPollTimeout,
	}, baseLogger)
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("orchestrator: started")
		orch.Run(ctx)
		log.Println("orchestrator: stopped")
	}()

	// Start MQ consumer if AMQP_URL is configured
	var mqConn *mq.Connection
	if cfg.AMQPURL != "" {
		mqConn = mq.NewConnection(cfg.AMQPURL)
		if err := mqConn.Connect(ctx); err != nil {
			log.Printf("mq: failed to connect: %v (continuing without MQ)", err)
			mqConn = nil
		} else {
			if err := mq.DeclareTopology(mqConn); err != nil {
				log.Fatalf("mq: topology declaration failed: %v", err)
			}
			consumer := mq.NewConsumer(mqConn, sqlStore, baseLogger)
			wg.Add(1)
			go func() {
				defer wg.Done()
				log.Println("mq: consumer started")
				if err := consumer.Run(ctx); err != nil {
					log.Printf("mq: consumer exited: %v", err)
				}
				log.Println("mq: consumer stopped")
			}()
		}
	}

	// Start HTTP server
	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	log.Println("shutting down...")

	if err := srv.Shutdown(); err != nil {
		log.Printf("server shutdown error: %v", err)
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
		log.Println("shutdown timed out waiting for goroutines")
	}
}
