package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"tezos-delegation-service/db"
	"tezos-delegation-service/internal/api"
	"tezos-delegation-service/internal/config"
	"tezos-delegation-service/internal/poller"
	"tezos-delegation-service/internal/store"
	"tezos-delegation-service/internal/tzkt"
)

func main() {
	cfg := config.Load()

	if err := db.Migrate(cfg.DB_DSN); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	dbConn, err := db.New(cfg.DB_DSN)
	if err != nil {
		log.Fatalf("cannot open the db: %v", err)
	}
	defer func(dbConn *sql.DB) {
		err := dbConn.Close()
		if err != nil {
			log.Printf("cannot close the db: %v", err)
		}
	}(dbConn)

	dbConn.SetMaxOpenConns(25)
	dbConn.SetMaxIdleConns(5)
	dbConn.SetConnMaxLifetime(5 * time.Minute)
	dbConn.SetConnMaxIdleTime(10 * time.Minute)

	if err := dbConn.Ping(); err != nil {
		log.Fatalf("cannot ping db: %v", err)
	}

	delegationStore := store.NewDelegationStore(dbConn)
	tzktClient := tzkt.NewClient(cfg.TzktBaseURL, cfg.HTTPClientTimeout)

	p := poller.NewPoller(poller.Config{
		Store:        delegationStore,
		Client:       tzktClient,
		BatchSize:    cfg.PollerBatchSize,
		PollInterval: cfg.PollerInterval,
		GenesisStart: time.Date(2018, 6, 30, 0, 0, 0, 0, time.UTC),
		MaxBackoff:   2 * time.Minute,
		Logger:       log.Default(),
	})

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      api.NewRouter(delegationStore, dbConn),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, gCtx := errgroup.WithContext(ctx)

	// Starting the poller in the background
	g.Go(func() error {
		log.Println("Starting poller...")
		if err := p.Run(gCtx); err != nil {
			return fmt.Errorf("poller error: %w", err)
		}
		log.Println("Poller stopped gracefully")
		return nil
	})

	g.Go(func() error {
		log.Printf("HTTP server listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("http server error: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		log.Println("Shutdown signal received, gracefully stopping...")

		// Shutdown HTTP server with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("http shutdown error: %w", err)
		}
		log.Println("HTTP server stopped gracefully")
		return nil
	})

	if err := g.Wait(); err != nil {
		log.Printf("Service stopped with error: %v", err)
		os.Exit(1)
	}

	log.Println("Service stopped gracefully")
}
