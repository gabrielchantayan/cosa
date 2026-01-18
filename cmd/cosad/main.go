// Command cosad is the Cosa daemon process.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"cosa/internal/config"
	"cosa/internal/daemon"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Check if already running
	if daemon.IsRunning(cfg.SocketPath) {
		fmt.Fprintln(os.Stderr, "Daemon is already running")
		os.Exit(1)
	}

	server, err := daemon.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	fmt.Printf("Cosa daemon v%s started (pid: %d)\n", config.Version, os.Getpid())
	fmt.Printf("Listening on %s\n", cfg.SocketPath)

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		server.Stop()
	}()

	server.Wait()
	fmt.Println("Daemon stopped")
}
