package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"bitwarden-reader/internal/k8s"
	"bitwarden-reader/internal/reader"

	"github.com/gin-gonic/gin"
)

// webHandler renders the HTML template with secret data
func (s *Server) webHandler(c *gin.Context) {
	ctx := c.Request.Context()
	secrets, err := reader.ReadSecrets(ctx, s.config.SecretNames, s.config.PodNamespace, s.k8sClients)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"Error":      err.Error(),
			"PodName":    s.config.PodName,
			"Namespace":  s.config.PodNamespace,
			"AppTitle":   s.config.AppTitle,
			"AppVersion": s.config.AppVersion,
		})
		return
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Secrets":     secrets,
		"TotalSecrets": countFoundSecrets(secrets),
		"PodName":     s.config.PodName,
		"Namespace":   s.config.PodNamespace,
		"AppTitle":    s.config.AppTitle,
		"AppVersion":  s.config.AppVersion,
		"ShowValues":  s.config.ShowSecretValues,
	})
}

// apiSecretsHandler returns JSON response with all secrets
func (s *Server) apiSecretsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	secrets, err := reader.ReadSecrets(ctx, s.config.SecretNames, s.config.PodNamespace, s.k8sClients)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secrets":    secrets,
		"namespace":  s.config.PodNamespace,
		"totalFound": countFoundSecrets(secrets),
		"timestamp":  time.Now().Format(time.RFC3339),
	})
}

// triggerSyncRequest represents the request body for trigger sync
type triggerSyncRequest struct {
	SecretNames []string `json:"secretNames,omitempty"`
}

// triggerSyncHandler patches CRD annotations to trigger sync
func (s *Server) triggerSyncHandler(c *gin.Context) {
	// Check if Kubernetes clients are available
	if s.k8sClients == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Kubernetes client not available - running in standalone mode",
		})
		return
	}

	ctx := c.Request.Context()

	var req triggerSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.SecretNames = s.config.SecretNames
	}

	if len(req.SecretNames) == 0 {
		req.SecretNames = s.config.SecretNames
	}

	var errors []string
	var successes []string

	for _, secretName := range req.SecretNames {
		secretName = strings.TrimSpace(secretName)
		if secretName == "" {
			continue
		}

		crdName := secretName
		err := k8s.TriggerSync(ctx, crdName, s.config.PodNamespace, s.k8sClients.DynamicClient)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", secretName, err))
		} else {
			successes = append(successes, secretName)
		}
	}

	if len(errors) > 0 {
		c.JSON(http.StatusPartialContent, gin.H{
			"successes": successes,
			"errors":    errors,
		})
		return
	}

	s.broadcastSecrets()

	c.JSON(http.StatusOK, gin.H{
		"message":   "Sync triggered successfully",
		"successes": successes,
	})
}

// healthHandler returns health check status
func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"version": s.config.AppVersion,
	})
}
