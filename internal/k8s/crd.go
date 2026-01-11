package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
)

// BitwardenSecretGVR is the GroupVersionResource for BitwardenSecret CRD
var BitwardenSecretGVR = schema.GroupVersionResource{
	Group:    "bitwarden-secrets-operator.io",
	Version:  "v1",
	Resource: "bitwardensecrets",
}

// CRDInfo holds information extracted from a BitwardenSecret CRD
type CRDInfo struct {
	CRDFound              bool
	LastSuccessfulSync    string
	SyncStatus            string
	SyncReason            string
	SyncMessage           string
	CRDCreationTime       string
}

// GetBitwardenSecretCRD retrieves a BitwardenSecret CRD and extracts sync information
func GetBitwardenSecretCRD(ctx context.Context, name, namespace string, dynamicClient dynamic.Interface) (*CRDInfo, error) {
	info := &CRDInfo{
		CRDFound: false,
	}

	// Get the CRD
	unstructuredObj, err := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return info, nil // CRD not found, but not an error
		}
		return nil, err
	}

	info.CRDFound = true

	// Extract metadata.creationTimestamp
	if creationTimestamp, found, err := unstructured.NestedString(unstructuredObj.Object, "metadata", "creationTimestamp"); err == nil && found {
		info.CRDCreationTime = creationTimestamp
	}

	// Extract status.lastSuccessfulSyncTime
	if lastSync, found, err := unstructured.NestedString(unstructuredObj.Object, "status", "lastSuccessfulSyncTime"); err == nil && found {
		info.LastSuccessfulSync = lastSync
	}

	// Extract status.conditions array
	conditions, found, err := unstructured.NestedSlice(unstructuredObj.Object, "status", "conditions")
	if err == nil && found {
		for _, condition := range conditions {
			conditionMap, ok := condition.(map[string]interface{})
			if !ok {
				continue
			}

			// Check if this is a SuccessfulSync condition
			conditionType, found, err := unstructured.NestedString(conditionMap, "type")
			if err != nil || !found || conditionType != "SuccessfulSync" {
				continue
			}

			// Extract status
			if status, found, err := unstructured.NestedString(conditionMap, "status"); err == nil && found {
				info.SyncStatus = status
			}

			// Extract reason
			if reason, found, err := unstructured.NestedString(conditionMap, "reason"); err == nil && found {
				info.SyncReason = reason
			}

			// Extract message
			if message, found, err := unstructured.NestedString(conditionMap, "message"); err == nil && found {
				info.SyncMessage = message
			}

			// Found the SuccessfulSync condition, break
			break
		}
	}

	return info, nil
}

// PatchCRDAnnotation patches the BitwardenSecret CRD with new annotations to trigger sync
func PatchCRDAnnotation(ctx context.Context, name, namespace string, annotations map[string]string, dynamicClient dynamic.Interface) error {
	// Get current CRD
	unstructuredObj, err := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get CRD: %w", err)
	}

	// Get current annotations
	currentAnnotations, found, err := unstructured.NestedStringMap(unstructuredObj.Object, "metadata", "annotations")
	if err != nil {
		return fmt.Errorf("failed to get current annotations: %w", err)
	}

	if !found || currentAnnotations == nil {
		currentAnnotations = make(map[string]string)
	}

	// Merge new annotations
	for key, value := range annotations {
		currentAnnotations[key] = value
	}

	// Create patch
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": currentAnnotations,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal patch: %w", err)
	}

	// Apply patch
	_, err = dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Patch(
		ctx,
		name,
		types.MergePatchType,
		patchBytes,
		metav1.PatchOptions{},
	)

	if err != nil {
		return fmt.Errorf("failed to patch CRD: %w", err)
	}

	return nil
}

// TriggerSync patches the CRD with force-sync annotation
func TriggerSync(ctx context.Context, name, namespace string, dynamicClient dynamic.Interface) error {
	annotations := map[string]string{
		"bitwarden-secrets-operator.io/force-sync": time.Now().Format(time.RFC3339),
	}
	return PatchCRDAnnotation(ctx, name, namespace, annotations, dynamicClient)
}
