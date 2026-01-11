package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bitwarden-reader/internal/config"
	"bitwarden-reader/internal/k8s"
	"bitwarden-reader/internal/server"
)

func main() {
	// Initialize configuration
	cfg := config.LoadConfig()

	// Setup Kubernetes clients (optional - can be nil for standalone mode)
	k8sClients, err := k8s.NewK8sClient()
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}
	if k8sClients == nil {
		log.Println("WARNING: Running in standalone mode - Kubernetes features will be limited")
		log.Println("To enable Kubernetes features, ensure kubeconfig is available or run in-cluster")
	}

	// Create server instance
	srv := server.NewServer(cfg, k8sClients)

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	log.Println("Server started successfully")
	log.Printf("Listening on port %d", cfg.Port)

	// Wait for interrupt signal
	<-quit
	log.Println("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
		return
	}

	log.Println("Server exited")
}
