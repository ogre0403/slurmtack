package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/slurmtack/slurmtack/internal/api"
	"github.com/slurmtack/slurmtack/internal/config"
	"github.com/slurmtack/slurmtack/internal/service"
	"github.com/slurmtack/slurmtack/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	sqlStore, err := store.NewSQLiteStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer sqlStore.Close()

	svc := service.NewSwitchService(sqlStore)
	srv := api.NewServer(cfg.ListenAddr, cfg.APIToken, sqlStore, svc)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := srv.Start(); err != nil {
			log.Fatalf("server: %v", err)
		}
	}()

	<-sigCh
	log.Println("shutting down...")
	if err := srv.Shutdown(); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
