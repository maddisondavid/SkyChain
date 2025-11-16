package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var (
		configPath string
		verbose    bool
	)

	flag.StringVar(&configPath, "config", "", "path to simulator configuration file (YAML or JSON)")
	flag.BoolVar(&verbose, "verbose", false, "enable verbose logging")
	flag.Parse()

	if configPath == "" {
		log.Fatal("--config is required")
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	infoLogger := log.New(os.Stdout, "[sim] ", log.LstdFlags)
	errorLogger := log.New(os.Stderr, "[sim] ", log.LstdFlags)

	infoLogger.Printf("loaded configuration from %s (%d devices)", cfg.Source(), cfg.Devices)

	sim, err := NewSimulator(cfg, infoLogger, errorLogger, verbose)
	if err != nil {
		log.Fatalf("create simulator: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	infoLogger.Printf("starting simulator targeting %s", cfg.Endpoint)
	sim.Run(ctx)
	infoLogger.Println("simulator stopped")
}
