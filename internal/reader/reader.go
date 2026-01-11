package reader

import (
	"context"
	"fmt"
	"log"
	"strings"

	"bitwarden-reader/internal/k8s"
)

// SecretInfo holds information about a Kubernetes secret and its sync status
type SecretInfo struct {
	Name     string
	Found    bool
	Keys     map[string]string
	SyncInfo SyncInfo
	Error    string
}

// SyncInfo holds synchronization information from the CRD
type SyncInfo struct {
	CRDFound            bool
	LastSuccessfulSync  string
	K8sSecretSyncTime   string
	SyncStatus          string
	SyncReason          string
	SyncMessage         string
	CRDCreationTime     string
}

// ReadSecrets reads all specified secrets and combines them with CRD sync information
func ReadSecrets(ctx context.Context, secretNames []string, namespace string, k8sClients *k8s.K8sClients) ([]SecretInfo, error) {
	var secrets []SecretInfo

	// Handle standalone mode (no Kubernetes clients)
	if k8sClients == nil {
		for _, secretName := range secretNames {
			secretName = strings.TrimSpace(secretName)
			if secretName == "" {
				continue
			}
			secrets = append(secrets, SecretInfo{
				Name:     secretName,
				Found:    false,
				Keys:     make(map[string]string),
				SyncInfo: SyncInfo{},
				Error:    "Kubernetes client not available - running in standalone mode",
			})
		}
		return secrets, nil
	}

	for _, secretName := range secretNames {
		secretName = strings.TrimSpace(secretName)
		if secretName == "" {
			continue
		}

		secretInfo := SecretInfo{
			Name:     secretName,
			Found:    false,
			Keys:     make(map[string]string),
			SyncInfo: SyncInfo{},
		}

		// Read Kubernetes Secret
		secret, err := k8s.ReadSecret(ctx, secretName, namespace, k8sClients.Clientset)
		if err != nil {
			if k8s.IsSecretNotFound(err) {
				secretInfo.Error = fmt.Sprintf("Secret '%s' not found", secretName)
			} else {
				secretInfo.Error = fmt.Sprintf("Error reading secret: %v", err)
			}
			secrets = append(secrets, secretInfo)
			continue
		}

		secretInfo.Found = true

		// Decode secret data
		secretInfo.Keys = k8s.DecodeSecretData(secret.Data)

		// Extract sync-time annotation
		secretInfo.SyncInfo.K8sSecretSyncTime = k8s.GetSecretSyncTime(secret)

		// If secret name starts with "bw-", get CRD info (remove "bw-" prefix)
		if strings.HasPrefix(secretName, "bw-") {
			crdName := strings.TrimPrefix(secretName, "bw-")
			readCRDInfo(ctx, crdName, namespace, secretName, k8sClients, &secretInfo)
		}

		secrets = append(secrets, secretInfo)
	}

	return secrets, nil
}

// readCRDInfo reads CRD information for a secret and updates the secretInfo
func readCRDInfo(ctx context.Context, crdName, namespace, secretName string, k8sClients *k8s.K8sClients, secretInfo *SecretInfo) {
	// Only try to read CRD if dynamic client is available
	if k8sClients.DynamicClient == nil {
		log.Printf("WARNING: DynamicClient is nil, cannot read CRD for secret %s", secretName)
		secretInfo.SyncInfo.SyncMessage = "DynamicClient not initialized"
		return
	}

	log.Printf("Reading CRD for secret %s (CRD name: %s, namespace: %s)", secretName, crdName, namespace)
	crdInfo, err := k8s.GetBitwardenSecretCRD(ctx, crdName, namespace, k8sClients.DynamicClient)
	if err != nil {
		log.Printf("ERROR: Failed to read CRD %s: %v", crdName, err)
		secretInfo.SyncInfo.SyncMessage = fmt.Sprintf("Error reading CRD: %v", err)
		return
	}

	log.Printf("CRD read result for %s: CRDFound=%v, SyncMessage=%s", crdName, crdInfo.CRDFound, crdInfo.SyncMessage)
	secretInfo.SyncInfo.CRDFound = crdInfo.CRDFound
	secretInfo.SyncInfo.LastSuccessfulSync = crdInfo.LastSuccessfulSync
	secretInfo.SyncInfo.SyncStatus = crdInfo.SyncStatus
	secretInfo.SyncInfo.SyncReason = crdInfo.SyncReason
	secretInfo.SyncInfo.SyncMessage = crdInfo.SyncMessage
	secretInfo.SyncInfo.CRDCreationTime = crdInfo.CRDCreationTime
}
