package utils

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	Constants "Detective/Constants"

	"helm.sh/helm/v3/pkg/action"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

// Reuse a single insecure HTTP client across the process to avoid repeated
// transport allocations and allow connection reuse (keep-alive).
var insecureTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

var insecureHTTPClient = &http.Client{Transport: insecureTransport}

// GetInsecureHTTPClient returns a shared HTTP client configured to skip TLS
// verification. Re-using this client reduces allocations and speeds up
// multiple sequential requests.
func GetInsecureHTTPClient() *http.Client {
	return insecureHTTPClient
}

// ParseJSON unmarshals raw JSON bytes into an interface{} and avoids an
// intermediate string/[]byte conversion that was present across callers.
func ParseJSON(data []byte) (interface{}, error) {
	var result interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON bytes: %w", err)
	}
	return result, nil
}

func ParseJSONString(jsonString string) (interface{}, error) {
	var result interface{}

	byteData := []byte(jsonString)
	err := json.Unmarshal(byteData, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON string: %w", err)
	}

	return result, nil
}

func FindHelmReleaseByChart(kubeconfigPath, targetChartVersion string) (string, string, error) {
	actionConfig := new(action.Configuration)
	configFlags := genericclioptions.NewConfigFlags(true) // 'true' uses persistent flags

	// Set the kubeconfig path directly on the flags object.
	configFlags.KubeConfig = &kubeconfigPath
	err := actionConfig.Init(configFlags, "", os.Getenv("HELM_DRIVER"), log.Printf)
	if err != nil {
		return "", "", fmt.Errorf("failed to initialize Helm action config: %w", err)
	}

	listAction := action.NewList(actionConfig)
	listAction.AllNamespaces = true
	listAction.SetStateMask()

	releases, err := listAction.Run()
	if err != nil {
		return "", "", fmt.Errorf("failed to run 'helm list' action: %w", err)
	}

	if len(releases) == 0 {
		return "", "", fmt.Errorf("no deployed Helm releases found in any namespace")
	}

	for _, rel := range releases {
		chartNameWithVersion := fmt.Sprintf("%s-%s", rel.Chart.Name(), rel.Chart.Metadata.Version)

		if chartNameWithVersion == targetChartVersion {
			log.Printf("✅ Release Name: '%s', Namespace: '%s'", rel.Name, rel.Namespace)
			return rel.Name, rel.Namespace, nil
		}
	}

	return "", "", fmt.Errorf("❌ no deployed release found for chart '%s'", targetChartVersion)
}

func TriggerPostRequestAndGetToken(serviceIP string) (string, error) {
	url := "https://" + serviceIP + ":9001/user"
	jsonData := `{"password":"Robin123","username":"robin"}`
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("POST", url, strings.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rakuten-internal", "user")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	token := resp.Header.Get("X-Rakuten-Token")
	if token == "" {
		return "", fmt.Errorf("header 'X-Rakuten-Token' not found in the response")
	}

	return token, nil
}

// It checks both the LoadBalancer Ingress status and the ExternalIPs spec field.
func GetExternalIPForService(clientset *kubernetes.Clientset, namespace, serviceName string) (string, error) {
	// log.Printf("🔎 Attempting to get service '%s' in namespace '%s'...", serviceName, namespace)

	// Get the service object from the cluster
	service, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("❌ failed to get service '%s' in namespace '%s': %w", serviceName, namespace, err)
	}

	// log.Printf("✅ Successfully retrieved service '%s'. Checking for external IP.", serviceName)
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		ingress := service.Status.LoadBalancer.Ingress[0]
		if ingress.IP != "" {
			log.Printf("✅ Found IP in LoadBalancer Ingress status: %s", ingress.IP)
			return ingress.IP, nil
		}
		// Fallback to hostname if IP is not available
		if ingress.Hostname != "" {
			log.Printf("✅ Found Hostname in LoadBalancer Ingress status: %s", ingress.Hostname)
			return ingress.Hostname, nil
		}
	}

	if len(service.Spec.ExternalIPs) > 0 {
		ip := service.Spec.ExternalIPs[0]
		log.Print("✅ Found IP in External IPs spec: " + ip + Constants.TwoNewLines)
		return ip, nil
	}
	return "", fmt.Errorf("❌ no external IP found for service '%s' (it might be <pending> or not exposed)", serviceName)
}
