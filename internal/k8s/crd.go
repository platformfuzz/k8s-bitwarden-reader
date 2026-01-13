package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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

// isAPIDiscoveryError checks if an error indicates API discovery failure
func isAPIDiscoveryError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "the server could not find the requested resource") ||
		strings.Contains(errMsg, "could not find the requested resource") ||
		strings.Contains(errMsg, "no matches for kind")
}

// checkAPIDiscovery verifies API discovery by attempting to list resources
func checkAPIDiscovery(ctx context.Context, namespace string, dynamicClient dynamic.Interface) error {
	_, listErr := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).List(ctx, metav1.ListOptions{Limit: 1})
	if listErr != nil {
		if isAPIDiscoveryError(listErr) {
			return listErr
		}
		// If it's a permission error, continue to try Get() anyway
		if !errors.IsForbidden(listErr) {
			log.Printf("List check failed (non-forbidden): %v, continuing with Get()", listErr)
		}
	}
	return nil
}

// handleNotFoundError handles 404 errors by trying cluster-scoped access
func handleNotFoundError(ctx context.Context, name, namespace string, dynamicClient dynamic.Interface) (*CRDInfo, error) {
	log.Printf("CRD not found (404): %s/%s in namespace %s, trying cluster-scoped access", BitwardenSecretGVR.Group, name, namespace)

	// Try cluster-scoped access
	unstructuredObj, err := dynamicClient.Resource(BitwardenSecretGVR).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return extractCRDInfo(unstructuredObj, name, namespace, "cluster-scoped"), nil
	}

	// Cluster-scoped also failed
	if errors.IsNotFound(err) {
		log.Printf("CRD not found: %s/%s (tried namespace %s and cluster-scoped)", BitwardenSecretGVR.Group, name, namespace)
		return &CRDInfo{
			CRDFound:    false,
			SyncMessage: fmt.Sprintf("CRD not found: %s", name),
		}, nil
	}

	// Cluster-scoped failed with other error
	log.Printf("Error reading CRD %s/%s (cluster-scoped): %v", BitwardenSecretGVR.Group, name, err)
	return &CRDInfo{
		CRDFound:    false,
		SyncMessage: fmt.Sprintf("Failed to get CRD (cluster-scoped): %v", err),
	}, nil
}

// handleGetError processes errors from Get() operation
func handleGetError(ctx context.Context, name, namespace string, err error, dynamicClient dynamic.Interface) (*CRDInfo, error) {
	errMsg := err.Error()
	log.Printf("ERROR reading CRD %s/%s in namespace %s: %v (type: %T, message: %s)",
		BitwardenSecretGVR.Group, name, namespace, err, err, errMsg)

	// Check for API discovery errors first
	if isAPIDiscoveryError(err) {
		log.Printf("API resource discovery issue for %s/%s: %v", BitwardenSecretGVR.Group, name, err)
		return &CRDInfo{
			CRDFound:    false,
			SyncMessage: fmt.Sprintf("API group '%s' not discoverable. CRD may not be installed or API server hasn't discovered it yet. Error: %v", BitwardenSecretGVR.Group, err),
		}, nil
	}

	// Check if it's a "not found" error (404)
	if errors.IsNotFound(err) {
		return handleNotFoundError(ctx, name, namespace, dynamicClient)
	}

	// Check for permission errors
	if errors.IsForbidden(err) {
		log.Printf("Permission denied accessing CRD %s/%s: %v", BitwardenSecretGVR.Group, name, err)
		return &CRDInfo{
			CRDFound:    false,
			SyncMessage: fmt.Sprintf("Permission denied accessing CRD %s. Check RBAC permissions. Error: %v", name, err),
		}, nil
	}

	// Check for other API-related errors
	if errors.IsMethodNotSupported(err) || errors.IsInvalid(err) {
		log.Printf("API group/resource issue: %v", err)
		return &CRDInfo{
			CRDFound:    false,
			SyncMessage: fmt.Sprintf("API group/resource issue: %v", err),
		}, nil
	}

	// For unexpected errors, still return info with message (don't fail completely)
	errorMsg := fmt.Sprintf("Failed to get CRD: %v", err)
	log.Printf("Unexpected error reading CRD %s/%s in namespace %s: %s", BitwardenSecretGVR.Group, name, namespace, errorMsg)
	return &CRDInfo{
		CRDFound:    false,
		SyncMessage: errorMsg,
	}, nil
}

// GetBitwardenSecretCRD retrieves a BitwardenSecret CRD and extracts sync information
// Always returns (info, nil) to ensure SyncMessage is set for error cases
func GetBitwardenSecretCRD(ctx context.Context, name, namespace string, dynamicClient dynamic.Interface) (*CRDInfo, error) {
	info := &CRDInfo{
		CRDFound: false,
	}

	// Validate inputs
	if dynamicClient == nil {
		log.Printf("ERROR: DynamicClient is nil, cannot read CRD %s/%s", namespace, name)
		info.SyncMessage = "DynamicClient not initialized"
		return info, nil
	}

	if name == "" {
		log.Printf("ERROR: CRD name is empty")
		info.SyncMessage = "CRD name is empty"
		return info, nil
	}

	if namespace == "" {
		log.Printf("ERROR: Namespace is empty for CRD %s", name)
		info.SyncMessage = "Namespace is empty"
		return info, nil
	}

	log.Printf("Attempting to get CRD: group=%s, version=%s, resource=%s, name=%s, namespace=%s",
		BitwardenSecretGVR.Group, BitwardenSecretGVR.Version, BitwardenSecretGVR.Resource, name, namespace)

	// First, try to verify API discovery by listing resources (this helps refresh discovery cache)
	if apiErr := checkAPIDiscovery(ctx, namespace, dynamicClient); apiErr != nil {
		log.Printf("API discovery failed for group %s: %v", BitwardenSecretGVR.Group, apiErr)
		info.SyncMessage = fmt.Sprintf("API group '%s' not discoverable. CRD may not be installed or API server hasn't discovered it yet. Error: %v", BitwardenSecretGVR.Group, apiErr)
		return info, nil
	}

	// Try namespace-scoped access first
	unstructuredObj, err := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return extractCRDInfo(unstructuredObj, name, namespace, "namespace-scoped"), nil
	}

	// Handle the error
	return handleGetError(ctx, name, namespace, err, dynamicClient)
}

// extractCRDInfo extracts all information from a CRD unstructured object
func extractCRDInfo(unstructuredObj *unstructured.Unstructured, name, namespace, scope string) *CRDInfo {
	info := &CRDInfo{
		CRDFound: true,
	}
	extractMetadata(unstructuredObj, info)
	extractStatusFields(unstructuredObj, info)
	extractConditions(unstructuredObj, info)
	log.Printf("Successfully read CRD %s/%s (%s): CRDFound=%v, LastSync=%s, Status=%s",
		BitwardenSecretGVR.Group, name, scope, info.CRDFound, info.LastSuccessfulSync, info.SyncStatus)
	return info
}

// PatchCRDAnnotation patches the BitwardenSecret CRD with new annotations to trigger sync
func PatchCRDAnnotation(ctx context.Context, name, namespace string, annotations map[string]string, dynamicClient dynamic.Interface) error {
	if dynamicClient == nil {
		return fmt.Errorf("dynamicClient is nil")
	}

	// Try namespace-scoped first, then cluster-scoped
	unstructuredObj, err := dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	isClusterScoped := false
	if err != nil {
		if errors.IsNotFound(err) {
			// Try cluster-scoped
			unstructuredObj, err = dynamicClient.Resource(BitwardenSecretGVR).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get CRD (tried namespace and cluster-scoped): %w", err)
			}
			isClusterScoped = true
		} else {
			return fmt.Errorf("failed to get CRD: %w", err)
		}
	}

	// Get and merge annotations
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

	// Apply patch (namespace-scoped or cluster-scoped)
	if isClusterScoped {
		_, err = dynamicClient.Resource(BitwardenSecretGVR).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = dynamicClient.Resource(BitwardenSecretGVR).Namespace(namespace).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	}

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
