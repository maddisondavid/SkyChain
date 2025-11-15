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

	"github.com/skychain/skychain/pkg/chain"
	"github.com/skychain/skychain/pkg/node"
)

func main() {
	var (
		dataPath  = flag.String("data", "data/chain.json", "path to the chain json file")
		addr      = flag.String("addr", ":8080", "http listen address")
		interval  = flag.Duration("interval", 10*time.Second, "block sealing interval")
		validator = flag.String("validator", "skychain-validator", "validator identifier")
		secret    = flag.String("secret", "skychain-local-secret", "validator secret for signing")
	)
	flag.Parse()

	ledger, err := chain.LoadOrCreate(*dataPath, *validator, *secret)
	if err != nil {
		log.Fatalf("load chain: %v", err)
	}

	skyNode, err := node.NewNode(ledger, *dataPath, *interval)
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

	if err := ledger.SaveToFile(*dataPath); err != nil {
		log.Printf("finalize chain: %v", err)
	}
}
