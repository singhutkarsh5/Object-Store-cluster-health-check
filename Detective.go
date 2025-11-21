package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	Check "Detective/Checks"
	Constants "Detective/Constants"
	Utils "Detective/Utils"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	start := time.Now()
	Issues := []string{}
	log.Print(Constants.BoldGreen + "Starting Object Store Diagnose" + Constants.Reset + Constants.TwoNewLines)

	// Set up kubernetes client
	config, err := clientcmd.BuildConfigFromFlags("", filepath.Join(homedir(), ".kube", "config"))
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating clientset: %v", err)
	}

	// Identify Helm release and namespace
	releaseName, appNamespace, err := Utils.FindHelmReleaseByChart(filepath.Join(homedir(), ".kube", "config"), Constants.HelmChart)
	if err != nil {
		log.Fatalf("Error finding Helm release: %v", err)
	}

	serviceName := "ostore-gateway-server"
	if releaseName != appNamespace && releaseName != "ostore" {
		serviceName = releaseName + "-" + "ostore-gateway-server"
	}

	// Get External IP of the service
	serviceIP, err := Utils.GetExternalIPForService(clientset, appNamespace, serviceName)
	if err != nil {
		log.Fatalf("Error getting external IP for service: %v", err)
	}

	// Perform core cluster health check
	fmt.Print(Constants.BoldGreen + "[1/10] Running Core Kubernetes Health Check" + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	if err := Check.KubernetesHealth(clientset); err != nil {
		log.Fatalf("❌ Core Kubernetes health check FAILED: %v", err)
	}

	log.Print("✅ Core Kubernetes components are healthy." + Constants.TwoNewLines)

	// Define the list of required pod prefixes for the 'ostore' namespace
	requiredOstorePods := []string{
		releaseName + "-gateway",
		releaseName + "-cm",
		releaseName + "-agent",
		releaseName + "-dashboard",
		releaseName + "-dstore",
		releaseName + "-metrics",
		"yb-master",
		"yb-tserver",
	}

	fmt.Print(Constants.BoldGreen + "[2/10] Running Application Pod Check for namespace: " + appNamespace + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess := Check.AllPodsAreRunning(clientset, appNamespace, requiredOstorePods)
	if isSuccess != "Success" {
		log.Printf("Application pod check for namespace '%s' FAILED: %v", appNamespace, isSuccess)
		Issues = append(Issues, isSuccess)
	}

	log.Print("All required pods are present and healthy in namespace: " + appNamespace + Constants.TwoNewLines)
	fmt.Print(Constants.BoldGreen + "[3/10] Running PersistentVolume Check " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	if err := Check.LocalPVsAreBound(clientset); err != nil {
		log.Printf("❌ PersistentVolume check FAILED: %v", err)
		Issues = append(Issues, err.Error())
	}

	token, err := Utils.TriggerPostRequestAndGetToken(serviceIP)
	if err != nil {
		log.Fatalf("❌ POST request FAILED: %v", err)
	}

	fmt.Print(Constants.BoldGreen + "[4/10] Checking ObjectStore Version " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess = Check.OstoreVersion(token, serviceIP)
	if isSuccess != "Success" {
		log.Printf("❌ Unable to get the ObjectStore Version, Reason: %v", isSuccess)
		Issues = append(Issues, isSuccess)
	}

	fmt.Print(Constants.BoldGreen + "[5/10] Checking Disks Status " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess = Check.DiskStatus(token, serviceIP)
	if isSuccess != "Success" {
		log.Printf("❌ GET request for disk status FAILED: %v", isSuccess)
		Issues = append(Issues, isSuccess)
	}

	fmt.Print(Constants.BoldGreen + "[6/10] Checking Diskset Status " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess = Check.DisksetStatus(token, serviceIP)
	if isSuccess != "Success" {
		log.Printf("❌ GET request for diskset status FAILED: %v", isSuccess)
		Issues = append(Issues, isSuccess)
	}

	fmt.Print(Constants.BoldGreen + "[7/10] Checking Node Status " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess = Check.NodesStatus(token, serviceIP)
	if isSuccess != "Success" {
		log.Print(isSuccess)
		Issues = append(Issues, isSuccess)
	}

	fmt.Print(Constants.BoldGreen + "[8/10] Checking Replication Status " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess = Check.ReplicationStatus(token, serviceIP)
	if isSuccess != "Success" {
		log.Print(isSuccess)
		Issues = append(Issues, isSuccess)
	}

	fmt.Print(Constants.BoldGreen + "[9/10] Checking LDAP Status " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess = Check.LDAPStatus(token, serviceIP)
	if isSuccess != "Success" {
		log.Print(isSuccess)
		Issues = append(Issues, isSuccess)
	}

	fmt.Print(Constants.BoldGreen + "[10/10] Checking Ostore Cluster Health Status " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	isSuccess = Check.ClusterHealth(token, serviceIP)
	if isSuccess != "Success" {
		log.Print(isSuccess)
		Issues = append(Issues, isSuccess)
	}

	if len(Issues) > 0 {
		fmt.Print(Constants.BoldRed + "Issues detected during the health check:" + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
		for _, issue := range Issues {
			fmt.Print(Constants.FgRed + "- " + issue + Constants.Reset)
		}
	} else {
		fmt.Print(Constants.Newline + Constants.BoldGreen + "Overall check successful! Both the cluster and the Object Store application are healthy. " + Constants.Reset + Constants.Newline + Constants.Differentiator + Constants.TwoNewLines)
	}

	timeSince := time.Since(start)
	log.Print(Constants.BoldGreen + "Total Time taken: " + fmt.Sprint(timeSince) + Constants.Reset + Constants.Newline)
}

func homedir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return ""
}
