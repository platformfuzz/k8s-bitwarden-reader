package k8s

import (
	"context"
	"encoding/base64"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ReadSecret reads a Kubernetes Secret by name and namespace
func ReadSecret(ctx context.Context, name, namespace string, clientset kubernetes.Interface) (*corev1.Secret, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// DecodeSecretData decodes base64 encoded secret values
func DecodeSecretData(data map[string][]byte) map[string]string {
	decoded := make(map[string]string)
	for key, value := range data {
		decodedValue, err := base64.StdEncoding.DecodeString(string(value))
		if err != nil {
			// If decoding fails, use the raw value
			decoded[key] = string(value)
		} else {
			decoded[key] = string(decodedValue)
		}
	}
	return decoded
}

// IsSecretNotFound checks if an error is a "not found" error
func IsSecretNotFound(err error) bool {
	return errors.IsNotFound(err)
}

// GetSecretSyncTime extracts the sync-time annotation from a secret
func GetSecretSyncTime(secret *corev1.Secret) string {
	if secret.Annotations == nil {
		return ""
	}
	return secret.Annotations["bitwarden-secrets-operator.io/sync-time"]
}
