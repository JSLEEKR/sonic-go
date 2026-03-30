// Package main is the entry point for the sonic-go server.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JSLEEKR/sonic-go/internal/config"
	"github.com/JSLEEKR/sonic-go/pkg/channel"
	"github.com/JSLEEKR/sonic-go/pkg/index"
	"github.com/JSLEEKR/sonic-go/pkg/search"
	"github.com/JSLEEKR/sonic-go/pkg/store"
	"github.com/JSLEEKR/sonic-go/pkg/suggest"
)

var (
	version = "1.0.0"
)

func main() {
	configPath := flag.String("config", "", "path to config YAML file")
	showVersion := flag.Bool("version", false, "show version and exit")
	listenAddr := flag.String("addr", "", "listen address (overrides config)")
	dataDir := flag.String("data", "", "data directory (overrides config)")
	password := flag.String("password", "", "auth password (overrides config)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("sonic-go v%s\n", version)
		os.Exit(0)
	}

	cfg := config.DefaultConfig()
	if *configPath != "" {
		var err error
		cfg, err = config.LoadFromFile(*configPath)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
	}

	// CLI overrides
	if *listenAddr != "" {
		cfg.Channel.ListenAddr = *listenAddr
	}
	if *dataDir != "" {
		cfg.Store.DataDir = *dataDir
	}
	if *password != "" {
		cfg.Channel.AuthPassword = *password
	}

	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	if err := run(cfg); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func run(cfg *config.Config) error {
	// Initialize store
	s := store.New(cfg.Store.DataDir)
	if err := s.LoadFromDisk(); err != nil {
		log.Printf("warning: failed to load store from disk: %v", err)
	}

	// Initialize suggest trie
	t := suggest.NewTrie()

	// Initialize index engine
	idx := index.New(s, t, cfg.Store.RetainWordObjects)

	// Initialize search engine
	eng := search.New(s, t)

	// Initialize channel server
	srv := channel.NewServer(
		cfg.Channel.ListenAddr,
		cfg.Channel.AuthPassword,
		s, t, idx, eng,
		cfg.Channel.MaxBufferSize,
	)

	if err := srv.Start(); err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	log.Printf("sonic-go v%s started on %s", version, srv.Addr())

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down...")

	// Save store
	if err := s.SaveToDisk(); err != nil {
		log.Printf("warning: failed to save store: %v", err)
	}

	return srv.Stop()
}
