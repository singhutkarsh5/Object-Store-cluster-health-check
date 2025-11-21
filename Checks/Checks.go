package checks

import (
	Constants "Detective/Constants"
	Utils "Detective/Utils"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	kubeSystemNamespace = "kube-system"
	helmChart           = "ostore-1.5.0"
)

// ParseJSONString takes a JSON string and unmarshals it into a generic Go data structure.
// It returns an interface{} which can be a map[string]interface{} (for JSON objects)
// or a []interface{} (for JSON arrays), along with an error.

// getNodesStatus gives you the node status in the cluster
// CheckNodesStatus makes a GET request to the /node endpoint and verifies that all nodes are ONLINE.
func NodesStatus(token string, serviceIP string) string {
	url := fmt.Sprintf("https://%s:9001/node", serviceIP)
	// log.Printf("Triggering GET request to: %s", url)

	client := Utils.GetInsecureHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Sprintf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")
	req.Header.Set("x-rakuten-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("received non-successful HTTP status: %s. Body: %s", resp.Status, string(bodyBytes))
	}

	// --- THE FIX IS HERE ---

	// 1. Parse the JSON string into a generic interface{}.
	parsedJSON, err := Utils.ParseJSON(bodyBytes)
	if err != nil {
		return fmt.Sprintf("failed to parse JSON response: %v", err)
	}

	// 2. Assert the type to a slice of interfaces ([]interface{}), which corresponds to a JSON array.
	nodeList, ok := parsedJSON.([]interface{})
	if !ok {
		return fmt.Sprintf("unexpected JSON structure: expected an array of nodes, but got %T", parsedJSON)
	}

	log.Print(" Total number of Object Store Nodes: ", len(nodeList))

	// 3. Loop through each item in the slice.
	for i, item := range nodeList {
		// Each item should be an object (map[string]interface{}).
		nodeMap, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Sprintf("unexpected item in JSON array at index %d: expected an object", i)
		}

		// 4. Safely extract and check the 'health_str' field.
		healthStr, healthOK := nodeMap["status_str"].(string)
		nodeName, nameOK := nodeMap["name"].(string)

		if !healthOK || !nameOK {
			return "A node in the response is missing or has invalid 'health_str' or 'name' fields"
		}

		log.Printf("✅ Checking Node: %s | Health: '%s'", nodeName, healthStr)

		// 5. Perform the validation.
		if healthStr != "ACTIVE" {
			return fmt.Sprintf("node '%s' is not ACTIVE. Current health: '%s'", nodeName, healthStr)
		}
	}
	log.Print("All the Nodes are Active" + Constants.TwoNewLines)

	return "Success"
}

func ReplicationStatus(token string, serviceIP string) string {
	url := fmt.Sprintf("https://%s:9000/cluster_replication_config", serviceIP)
	// log.Printf("Triggering GET request to: %s", url)

	client := Utils.GetInsecureHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Sprintf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")
	req.Header.Set("x-rakuten-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %s", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("received non-successful HTTP status: %s. Body: %s", resp.Status, string(bodyBytes))
	}

	if string(bodyBytes) == "{}" {
		return "❌ Replication not set" + Constants.TwoNewLines
	}

	parsedJSON, err := Utils.ParseJSON(bodyBytes)
	if err != nil {
		return fmt.Sprintf("failed to parse JSON response: %s", err)
	}

	parsedJSONMap, ok := parsedJSON.(map[string]interface{})
	if !ok {
		return "unexpected JSON structure: expected an object at the top level"
	}

	replicatedCluster, ok := parsedJSONMap["ReplicatedClusters"].([]interface{})
	if !ok || len(replicatedCluster) == 0 {
		return "unexpected JSON structure: expected an object in 'ReplicatedCluster' array"
	}

	firstCluster, ok := replicatedCluster[0].(map[string]interface{})
	if !ok {
		return "unexpected JSON structure: expected an object in 'ReplicatedCluster' array"
	}

	health, ok := firstCluster["Health"].(string)
	if !ok {
		return "unexpected JSON structure: 'Health' field is missing or not a string"
	}

	if health != "ONLINE" {
		return fmt.Sprintf("Replication is configured but the health is not Online, current health: %s", health)
	}

	log.Print("✅ Replication is set" + Constants.TwoNewLines)

	return "Success"
}

// OstoreVersion gives you the objectStore version installed in the cluster
func OstoreVersion(token string, serviceIP string) string {
	url := fmt.Sprintf("https://%s:9001/version", serviceIP)
	// log.Printf("Triggering GET request to: %s", url)

	client := Utils.GetInsecureHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Sprintf("failed to create request: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")
	req.Header.Set("x-rakuten-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("failed to execute request: %s", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %s", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("received non-successful HTTP status: %s. Body: %s", resp.Status, string(bodyBytes))
	}
	log.Print("Object Store version is: " + string(bodyBytes) + Constants.TwoNewLines)

	return "Success"
}

// triggerPostRequest makes an insecure POST request and prints the full response.
func DisksetStatus(token string, serviceIP string) string {
	url := "https://" + serviceIP + ":9001/diskset?action=list"
	// log.Printf("Triggering GET request to: %s", url)

	client := Utils.GetInsecureHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Sprintf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")
	req.Header.Set("x-rakuten-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("failed to execute request: %s", err)
	}
	defer resp.Body.Close()

	// Read the body first to include it in potential error messages
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %s", err)
	}

	// Check for a successful status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("received non-successful HTTP status: %s. Body: %s", resp.Status, string(bodyBytes))
	}

	// Return the body as a string on success

	parsedJSON, err := Utils.ParseJSON(bodyBytes)
	if err != nil {
		return fmt.Sprintf("failed to parse JSON response: %s", err)
	}

	parsedJSONMap, ok := parsedJSON.(map[string]interface{})
	if !ok {
		return "unexpected JSON structure: expected an object at the top level"
	}
	disksets := parsedJSONMap["disksets"].([]interface{})
	log.Println("Total number of disksets on the cluster:", len(disksets))
	for _, j := range disksets {

		disksetHealth := j.(map[string]interface{})["health_str"]
		disksetID := j.(map[string]interface{})["id"]
		disksetStatus := j.(map[string]interface{})["status_str"]
		log.Printf("✅ Diskset ID: %v, Health : %v, Status: %v\n", disksetID, disksetHealth, disksetStatus)
		if disksetHealth != "HEALTHY" || disksetStatus != "ACTIVE" && disksetStatus != "REBUILDING" {
			return fmt.Sprintf("❌ Diskset ID %v is not healthy or active. Health: %v, Status: %v", disksetID, disksetHealth, disksetStatus)
		}
	}
	if len(disksets) == 0 {
		return "❌ There are no disksets present, User can not perform data operations\n"
	}
	log.Print("All the Diskset/Disksets are Healthy" + Constants.TwoNewLines)
	return "Success"
}

func DiskStatus(token string, serviceIP string) string {
	// ... (pasting the corrected function from above) ...
	url := fmt.Sprintf("https://%s:9001/disk", serviceIP)
	// log.Printf("Triggering GET request to: %s", url)

	client := Utils.GetInsecureHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Sprintf("failed to create request: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")
	req.Header.Set("x-rakuten-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("failed to execute request: %s", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %s", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("received non-successful HTTP status: %s. Body: %s", resp.Status, string(bodyBytes))
	}

	parsedJSON, err := Utils.ParseJSON(bodyBytes)
	if err != nil {
		return fmt.Sprintf("failed to parse JSON response: %s", err)
	}

	diskList, ok := parsedJSON.([]interface{})
	if !ok {
		return fmt.Sprintf("unexpected JSON structure: expected an array at the top level, but got %T", parsedJSON)
	}

	log.Print("Total number of disks present in the ObjectStore Cluster: ", len(diskList))
	if len(diskList) == 0 {
		return "❌ There are no disks present in the ObjectStore Cluster, A user can not perform data operations\n"
	}

	for i, item := range diskList {
		disk, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Sprintf("unexpected item in JSON array at index %d: expected an object", i)
		}

		healthStr := disk["health_str"].(string)
		statusStr := disk["status_str"].(string)
		diskID := disk["disk_id"]

		if healthStr != "ONLINE" {
			return fmt.Sprintf("❌  Disk with Id %0.f is unhealthy: expected ONLINE/OFFLINE, got health %s and status %s", diskID, healthStr, statusStr)
		}

		if statusStr != "IN_USE" && statusStr != "UNUSED" {
			return fmt.Sprintf("❌ Disk with Id %d has invalid status: expected IN_USE or UNUSED, got %s", diskID, statusStr)
		}
		log.Printf("✅ Disk ID: %v, Health: %s, Status: %s", diskID, healthStr, statusStr)
	}
	log.Print("Success! All the Disks are Healthy" + Constants.TwoNewLines)

	return "Success"
}

func LDAPStatus(token string, serviceIP string) string {
	url := fmt.Sprintf("https://%s:9001/idp?idp=ldap", serviceIP)
	// log.Printf("Triggering GET request to: %s", url)

	client := Utils.GetInsecureHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Sprintf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")
	req.Header.Set("x-rakuten-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("failed to execute request: %s", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %s", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("received non-successful HTTP status: %s. Body: %s", resp.Status, string(bodyBytes))
	}
	parsedJSON, err := Utils.ParseJSON(bodyBytes)
	if err != nil {
		return fmt.Sprintf("failed to parse JSON response: %s", err)
	}
	parsedJSONMap, ok := parsedJSON.(map[string]interface{})
	if !ok {
		return "unexpected JSON structure: expected an object at the top level" + Constants.TwoNewLines
	}
	status := parsedJSONMap["ldap_info"].(map[string]interface{})["status_str"]
	server_address := parsedJSONMap["ldap_info"].(map[string]interface{})["ldap_server_address"]
	if status == "DISABLED" && server_address == "" {
		return "❌ LDAP is not configured" + Constants.TwoNewLines
	}
	if status == "ENABLED" {
		log.Print("✅ LDAP is configured and Enabled" + Constants.TwoNewLines)
	}
	if status == "DISABLED" && server_address != "" {
		log.Print("⚠️ Ldap is Cconfigured but Disabled" + Constants.TwoNewLines)
	}
	return "Success"
}

func ClusterHealth(token string, serviceIP string) string {
	url := fmt.Sprintf("https://%s:9001/cluster_health", serviceIP)
	// log.Printf("Triggering GET request to: %s", url)
	client := Utils.GetInsecureHTTPClient()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Sprintf("failed to create request: %s", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")
	req.Header.Set("x-rakuten-token", token)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Sprintf("failed to execute request: %s", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("failed to read response body: %s", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Sprintf("received non-successful HTTP status: %s. Body: %s", resp.Status, string(bodyBytes))
	}
	parsedJSON, err := Utils.ParseJSON(bodyBytes)
	if err != nil {
		return fmt.Sprintf("failed to parse JSON response: %s", err)
	}
	parsedJSONMap, ok := parsedJSON.(map[string]interface{})
	if !ok {
		return "unexpected JSON structure: expected an object at the top level"
	}
	controlHealthStatus := parsedJSONMap["controlHealthStatus"]
	if controlHealthStatus != "Online" {
		return fmt.Sprintf("❌ Cluster health check failed: expected Online, got %s", controlHealthStatus)
	} else {
		log.Println("✅ Control Path is Online")
	}
	metadataHealthStatus := parsedJSONMap["metadataHealthStatus"]
	if metadataHealthStatus != "Online" {
		return fmt.Sprintf("❌ Cluster health check failed: expected Online, got %s", metadataHealthStatus)
	} else {
		log.Println("✅ Metadata store status is Online")
	}
	datapathHealthStatus := parsedJSONMap["datapathHealthStatus"]
	if datapathHealthStatus != "Online" {
		return fmt.Sprintf("❌ Cluster health check failed: expected Online, got %s", datapathHealthStatus)
	} else {
		log.Println("✅ Data Path is Online")
	}
	clusterStatus := parsedJSONMap["clusterHealthStatus"]
	if clusterStatus != "Online" {
		return fmt.Sprintf("❌ Cluster health check failed: expected Online, got %s", clusterStatus)
	} else {
		log.Print("✅ Cluster Health is Online" + Constants.TwoNewLines)
	}

	return "Success"
}

// CheckClusterHealth performs a series of checks against critical cluster components.
func KubernetesHealth(clientset *kubernetes.Clientset) error {
	log.Println(" Checking core component status...")
	componentStatuses, err := clientset.CoreV1().ComponentStatuses().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("❌ failed to list component statuses: %w", err)
	}
	for _, cs := range componentStatuses.Items {
		isHealthy := false
		for _, condition := range cs.Conditions {
			if condition.Type == "Healthy" && condition.Status == v1.ConditionTrue {
				isHealthy = true
				break
			}
		}
		if !isHealthy {
			return fmt.Errorf("component '%s' is not healthy. Conditions: %+v", cs.Name, cs.Conditions)
		}
		log.Printf("✅ Component '%s' is healthy.", cs.Name)
	}
	fmt.Print(Constants.TwoNewLines)
	log.Println(" Checking all Kubernetes cluster nodes are ready...")
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("❌ failed to list nodes: %w", err)
	}
	for _, node := range nodes.Items {
		isNodeReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady && condition.Status == v1.ConditionTrue {
				isNodeReady = true
				break
			}
		}
		if !isNodeReady {
			return fmt.Errorf("❌ node '%s' is not ready. Status: %+v", node.Name, node.Status.Conditions)
		}
		log.Printf("✅ Kubernetes Node '%s' is ready.", node.Name)
	}
	fmt.Print(Constants.TwoNewLines)
	log.Printf("Checking all pods in '%s' namespace...", kubeSystemNamespace)
	// For kube-system, we don't have a list of required pods, so we pass 'nil'.
	if isSuccess := AllPodsAreRunning(clientset, kubeSystemNamespace, nil); isSuccess != "Success" {
		return fmt.Errorf("health check for pods in '%s' failed: %s", kubeSystemNamespace, isSuccess)
	}

	return nil
}

// checkAllPodsAreRunning verifies that all pods are ready and that a specific list of required pods exists.
// It returns "Success" if all checks pass, otherwise it returns a descriptive error message.
func AllPodsAreRunning(clientset *kubernetes.Clientset, namespace string, requiredPodPrefixes []string) string {
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Sprintf("❌ failed to list pods in namespace %s: %s", namespace, err)
	}

	if len(pods.Items) == 0 && len(requiredPodPrefixes) > 0 {
		return fmt.Sprintf("❌ no pods found in namespace '%s', but required pods were expected", namespace)
	}

	// Create a map to track if we've found each required pod.
	foundPods := make(map[string]bool)
	// if requiredPodPrefixes != nil {
	for _, p := range requiredPodPrefixes {
		foundPods[p] = false // Initialize all as not found
	}
	// }

	// First, iterate through all pods to check their status and mark required pods as found.
	for _, pod := range pods.Items {
		// --- NEW Check 1: Pod must not be Terminating ---
		if pod.ObjectMeta.DeletionTimestamp != nil {
			return fmt.Sprintf("❌ pod '%s' is terminating", pod.Name)
		}

		// --- NEW Check 2: Pod must not be Evicted ---
		if pod.Status.Reason == "Evicted" {
			return fmt.Sprintf("❌ pod '%s' has been evicted. Check node status and resource limits", pod.Name)
		}

		// Ignore pods that have completed their lifecycle (like Jobs)
		if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
			log.Printf("  -> Skipping pod '%s' with status '%s'.", pod.Name, pod.Status.Phase)
			continue
		}

		// --- Check 3: Pod must be in Running phase ---
		if pod.Status.Phase != v1.PodRunning {
			return fmt.Sprintf("❌ pod '%s' is not in 'Running' phase. Current phase: '%s'", pod.Name, pod.Status.Phase)
		}

		// --- Check 4: All containers must be ready and not in a failure loop ---
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				// Provide specific, actionable error messages for common failure states.
				if containerStatus.State.Waiting != nil {
					reason := containerStatus.State.Waiting.Reason
					message := containerStatus.State.Waiting.Message
					// NEW: Specific checks for common errors
					if reason == "CrashLoopBackOff" || reason == "ImagePullBackOff" || reason == "ErrImagePull" {
						return fmt.Sprintf("❌ container '%s' in pod '%s' is not ready. Reason: %s - %s",
							containerStatus.Name, pod.Name, reason, message)
					}
					// Generic waiting message
					return fmt.Sprintf("❌ container '%s' in pod '%s' is in a waiting state. Reason: %s - %s",
						containerStatus.Name, pod.Name, reason, message)
				}

				// NEW: Check if the container has terminated with an error
				if containerStatus.State.Terminated != nil {
					return fmt.Sprintf("❌ container '%s' in pod '%s' has terminated with exit code %d. Reason: %s",
						containerStatus.Name, pod.Name, containerStatus.State.Terminated.ExitCode, containerStatus.State.Terminated.Reason)
				}

				// Fallback for any other non-ready state
				return fmt.Sprintf("❌ container '%s' in pod '%s' is not ready for an unknown reason", containerStatus.Name, pod.Name)
			}
		}

		// --- Check 5: Pod must be marked as Ready in its conditions ---
		isPodReady := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				isPodReady = true
				break
			}
		}
		if !isPodReady {
			return fmt.Sprintf("❌ pod '%s' is not ready. Check its readiness probes and conditions", pod.Name)
		}

		log.Printf("✅ Pod '%s' is running and ready.", pod.Name)

		// --- Check 6: Mark required pods as found ---

		for _, prefix := range requiredPodPrefixes {
			// Use the map to avoid re-checking already found prefixes
			if !foundPods[prefix] && strings.HasPrefix(pod.Name, prefix) {
				foundPods[prefix] = true
			}
		}

	}

	// --- Final Check: Verify all required pods were found ---
	if requiredPodPrefixes != nil {
		for prefix, found := range foundPods {
			if !found {
				return fmt.Sprint("❌ Following pod not found: " + prefix + Constants.TwoNewLines)
			}
		}
	}
	return "Success"
}

// CheckLocalPVsAreBound verifies that all PersistentVolumes with the 'local-pv-' prefix are in a 'Bound' state.
func LocalPVsAreBound(clientset *kubernetes.Clientset) error {
	pvList, err := clientset.CoreV1().PersistentVolumes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list PersistentVolumes: %w", err)
	}

	foundMatchingPV := false // Keep track if we find any PVs with the prefix

	// 2. Iterate through all PVs and check the ones with the 'local-pv-' prefix
	for _, pv := range pvList.Items {
		if strings.HasPrefix(pv.Name, "local-pv-") {
			foundMatchingPV = true
			log.Printf("✅ Checking PV: %-25s | Status: %s", pv.Name, pv.Status.Phase)

			// 3. Check if the status is 'Bound'
			if pv.Status.Phase != v1.VolumeBound {
				// 4. If not bound, return an error immediately
				return fmt.Errorf("❌ persistent volume '%s' is not in 'Bound' state. Current state: '%s'", pv.Name, pv.Status.Phase)
			}
		}
	}

	// Handle the case where no PVs with the prefix were found
	if !foundMatchingPV {
		log.Println("⚠️ No Local PersistentVolumes were found.")
	}
	log.Print(" Success! All Local PersistentVolumes are in the 'Bound' state." + Constants.TwoNewLines)

	return nil
}
