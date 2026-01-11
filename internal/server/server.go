package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"bitwarden-reader/internal/config"
	"bitwarden-reader/internal/k8s"
	"bitwarden-reader/internal/reader"

	"github.com/gin-gonic/gin"
)

// Server holds the HTTP server and its dependencies
type Server struct {
	router        *gin.Engine
	k8sClients    *k8s.K8sClients
	config        *config.Config
	hub           *Hub
	httpServer    *http.Server
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, k8sClients *k8s.K8sClients) *Server {
	// Set Gin mode
	if gin.Mode() == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Create WebSocket hub
	hub := newHub()
	go hub.run()

	server := &Server{
		router:     router,
		k8sClients: k8sClients,
		config:     cfg,
		hub:        hub,
	}

	// Register routes
	server.registerRoutes()

	// Load HTML templates
	server.router.LoadHTMLGlob("web/templates/*")

	// Start broadcasting secrets periodically
	go server.startBroadcasting()

	return server
}

// registerRoutes registers all HTTP routes
func (s *Server) registerRoutes() {
	// Static files
	s.router.Static("/static", "./web/static")

	// Web UI
	s.router.GET("/", s.webHandler)

	// API endpoints
	api := s.router.Group("/api/v1")
	{
		api.GET("/secrets", s.apiSecretsHandler)
		api.POST("/trigger-sync", s.triggerSyncHandler)
		api.GET("/health", s.healthHandler)
	}

	// WebSocket endpoint
	s.router.GET("/ws", s.wsHandler)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: s.router,
	}

	log.Printf("Starting server on port %d", s.config.Port)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// startBroadcasting starts a goroutine that broadcasts secret updates periodically
func (s *Server) startBroadcasting() {
	ticker := time.NewTicker(s.config.DashboardRefreshInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.broadcastSecrets()
	}
}

// broadcastSecrets broadcasts current secret state to all WebSocket clients
func (s *Server) broadcastSecrets() {
	ctx := context.Background()
	secrets, err := reader.ReadSecrets(ctx, s.config.SecretNames, s.config.PodNamespace, s.k8sClients)
	if err != nil {
		// Log error but still try to broadcast what we have
		log.Printf("Error reading secrets: %v", err)
	}

	totalFound := 0
	for _, secret := range secrets {
		if secret.Found {
			totalFound++
		}
	}

	message := map[string]interface{}{
		"secrets":    secrets,
		"namespace":  s.config.PodNamespace,
		"totalFound": totalFound,
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	// Add error message if in standalone mode
	if s.k8sClients == nil {
		message["error"] = "Kubernetes client not available - running in standalone mode"
	}

	s.hub.broadcastMessage(message)
}
