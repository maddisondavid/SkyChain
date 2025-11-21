package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/skychain/skychain/pkg/node"
	"github.com/skychain/skychain/pkg/registry"
	"github.com/skychain/skychain/pkg/storage"
)

func main() {
	var (
		dataPath   = flag.String("data", "data/chain.db", "path to the chain database file")
		addr       = flag.String("addr", ":8080", "http listen address")
		interval   = flag.Duration("interval", 10*time.Second, "block sealing interval")
		validator  = flag.String("validator", "skychain-validator", "validator identifier")
		secret     = flag.String("secret", "skychain-local-secret", "validator secret for signing")
		devicesCfg = flag.String("devices", "config/devices.json", "path to devices.json registry")
	)
	flag.Parse()

	reg, err := registry.Load(*devicesCfg)
	if err != nil {
		log.Fatalf("load device registry: %v", err)
	}

	store, err := storage.OpenFileBlockStore(*dataPath)
	if err != nil {
		log.Fatalf("open block store: %v", err)
	}
	defer store.Close()

	ledger, err := store.LoadChain(*validator, *secret)
	if err != nil {
		log.Fatalf("load chain: %v", err)
	}

	skyNode, err := node.NewNode(ledger, store, *interval, reg)
	if err != nil {
		log.Fatalf("create node: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	skyNode.Start(ctx)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      skyNode.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		log.Printf("skychain node listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	log.Println("shutting down node")
	cancel()
	skyNode.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown error: %v", err)
	}

	// Blocks are persisted incrementally, so no final flush is required.
}
