package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
	Group:    "k8s.bitwarden.com",
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

// extractMetadata extracts metadata fields from the CRD
func extractMetadata(unstructuredObj *unstructured.Unstructured, info *CRDInfo) {
	if creationTimestamp, found, err := unstructured.NestedString(unstructuredObj.Object, "metadata", "creationTimestamp"); err == nil && found {
		info.CRDCreationTime = creationTimestamp
	}
}

// extractStatusFields extracts status fields from the CRD
func extractStatusFields(unstructuredObj *unstructured.Unstructured, info *CRDInfo) {
	if lastSync, found, err := unstructured.NestedString(unstructuredObj.Object, "status", "lastSuccessfulSyncTime"); err == nil && found {
		info.LastSuccessfulSync = lastSync
	}
}

// extractConditionFields extracts condition fields from a condition map
func extractConditionFields(conditionMap map[string]interface{}, info *CRDInfo) {
	if status, found, err := unstructured.NestedString(conditionMap, "status"); err == nil && found {
		info.SyncStatus = status
	}
	if reason, found, err := unstructured.NestedString(conditionMap, "reason"); err == nil && found {
		info.SyncReason = reason
	}
	if message, found, err := unstructured.NestedString(conditionMap, "message"); err == nil && found {
		info.SyncMessage = message
	}
}

// extractConditions extracts condition information from the CRD
func extractConditions(unstructuredObj *unstructured.Unstructured, info *CRDInfo) {
	conditions, found, err := unstructured.NestedSlice(unstructuredObj.Object, "status", "conditions")
	if err != nil {
		log.Printf("Error extracting conditions slice: %v", err)
		return
	}
	if !found {
		log.Printf("No conditions found in CRD status")
		return
	}

	for i, condition := range conditions {
		conditionMap, ok := condition.(map[string]interface{})
		if !ok {
			log.Printf("Condition %d is not a map[string]interface{}", i)
			continue
		}

		conditionType, found, err := unstructured.NestedString(conditionMap, "type")
		if err != nil {
			log.Printf("Error extracting condition type: %v", err)
			continue
		}
		if !found {
			log.Printf("Condition %d has no type field", i)
			continue
		}
		if conditionType != "SuccessfulSync" {
			continue
		}

		extractConditionFields(conditionMap, info)
		break // Found the SuccessfulSync condition
	}
}

// GetBitwardenSecretCRD retrieves a BitwardenSecret CRD and extracts sync information
// Tries namespace-scoped access first, then cluster-scoped if that fails
func GetBitwardenSecretCRD(ctx context.Context, name, namespace string, dynamicClient dynamic.Interface) (*CRDInfo, error) {
	info := &CRDInfo{
		CRDFound: false,
	}

	if dynamicClient == nil {
		return nil, fmt.Errorf("dynamicClient is nil")
	}

	log.Printf("Attempting to get CRD: group=%s, version=%s, resource=%s, name=%s, namespace=%s",
		BitwardenSecretGVR.Group, BitwardenSecretGVR.Version, BitwardenSecretGVR.Resource, name, namespace)

	// Try namespace-scoped access first
	unstructuredObj, err := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// Success with namespace-scoped access
		info.CRDFound = true
		extractMetadata(unstructuredObj, info)
		extractStatusFields(unstructuredObj, info)
		extractConditions(unstructuredObj, info)
		log.Printf("Successfully read CRD %s/%s: CRDFound=%v, LastSync=%s, Status=%s",
			BitwardenSecretGVR.Group, name, info.CRDFound, info.LastSuccessfulSync, info.SyncStatus)
		return info, nil
	}

	// Handle error - try cluster-scoped if namespace-scoped failed with "not found"
	if !errors.IsNotFound(err) {
		return handleCRDAccessError(err, name, namespace, dynamicClient, ctx)
	}

	// Try cluster-scoped access
	log.Printf("CRD not found in namespace %s, trying cluster-scoped access", namespace)
	unstructuredObj, err = dynamicClient.Resource(BitwardenSecretGVR).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		log.Printf("Successfully found CRD %s/%s using cluster-scoped access", BitwardenSecretGVR.Group, name)
		info.CRDFound = true
		extractMetadata(unstructuredObj, info)
		extractStatusFields(unstructuredObj, info)
		extractConditions(unstructuredObj, info)
		log.Printf("Successfully read CRD %s/%s: CRDFound=%v, LastSync=%s, Status=%s",
			BitwardenSecretGVR.Group, name, info.CRDFound, info.LastSuccessfulSync, info.SyncStatus)
		return info, nil
	}

	// Cluster-scoped also failed
	if errors.IsNotFound(err) {
		log.Printf("CRD not found: %s/%s (tried namespace %s and cluster-scoped)", BitwardenSecretGVR.Group, name, namespace)
		return info, nil // CRD not found, but not an error
	}

	log.Printf("Error reading CRD %s/%s (cluster-scoped): %v", BitwardenSecretGVR.Group, name, err)
	return nil, fmt.Errorf("failed to get CRD %s/%s: %w", BitwardenSecretGVR.Group, name, err)

}

// handleCRDAccessError handles errors when accessing CRD (non-404 errors)
func handleCRDAccessError(err error, name, namespace string, dynamicClient dynamic.Interface, ctx context.Context) (*CRDInfo, error) {
	log.Printf("Error reading CRD %s/%s in namespace %s: %v", BitwardenSecretGVR.Group, name, namespace, err)

	// Check if it's an API discovery error
	errMsg := err.Error()
	if errors.IsNotFound(err) || errMsg == "the server could not find the requested resource" {
		log.Printf("API resource not found - checking if CRD is registered. Error: %v", err)
		// Try to list resources to verify API discovery
		_, listErr := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).List(ctx, metav1.ListOptions{Limit: 1})
		if listErr != nil {
			log.Printf("API discovery check failed: %v", listErr)
		}
	}

	return nil, fmt.Errorf("failed to get CRD %s/%s: %w", BitwardenSecretGVR.Group, name, err)
}

// PatchCRDAnnotation patches the BitwardenSecret CRD with new annotations to trigger sync
func PatchCRDAnnotation(ctx context.Context, name, namespace string, annotations map[string]string, dynamicClient dynamic.Interface) error {
	if dynamicClient == nil {
		return fmt.Errorf("dynamicClient is nil")
	}

	// Try namespace-scoped first, then cluster-scoped
	unstructuredObj, err := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Try cluster-scoped
			unstructuredObj, err = dynamicClient.Resource(BitwardenSecretGVR).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get CRD (tried namespace and cluster-scoped): %w", err)
			}
			// For cluster-scoped, patch without namespace
			return patchCRDAnnotationClusterScoped(ctx, name, annotations, dynamicClient, unstructuredObj)
		}
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

	// Apply patch (namespace-scoped)
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

// patchCRDAnnotationClusterScoped patches a cluster-scoped CRD
func patchCRDAnnotationClusterScoped(ctx context.Context, name string, annotations map[string]string, dynamicClient dynamic.Interface, unstructuredObj *unstructured.Unstructured) error {
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

	// Apply patch (cluster-scoped, no namespace)
	_, err = dynamicClient.Resource(BitwardenSecretGVR).Patch(
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
		"k8s.bitwarden.com/force-sync": time.Now().Format(time.RFC3339),
	}
	return PatchCRDAnnotation(ctx, name, namespace, annotations, dynamicClient)
}
