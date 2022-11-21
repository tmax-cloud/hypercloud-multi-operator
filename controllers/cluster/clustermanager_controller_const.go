package controllers

import "time"

const (
	requeueAfter10Second = 10 * time.Second
	requeueAfter20Second = 20 * time.Second
	requeueAfter30Second = 30 * time.Second
	requeueAfter1Minute  = 1 * time.Minute
)

type ClusterParameter struct {
	Namespace         string
	ClusterName       string
	MasterNum         int
	WorkerNum         int
	Owner             string
	KubernetesVersion string
	HyperAuthUrl      string
}

type AwsParameter struct {
	SshKey         string
	Region         string
	MasterType     string
	WorkerType     string
	MasterDiskSize int
	WorkerDiskSize int
}

type VsphereParameter struct {
	PodCidr             string
	VcenterIp           string
	VcenterId           string
	VcenterPassword     string
	VcenterThumbprint   string
	VcenterNetwork      string
	VcenterDataCenter   string
	VcenterDataStore    string
	VcenterFolder       string
	VcenterResourcePool string
	VcenterKcpIp        string
	VcenterCpuNum       int
	VcenterMemSize      int
	VcenterDiskSize     int
	VcenterTemplate     string
}

type VsphereUpgradeParameter struct {
	Namespace           string
	ClusterName         string
	VcenterIp           string
	VcenterThumbprint   string
	VcenterNetwork      string
	VcenterDataCenter   string
	VcenterDataStore    string
	VcenterFolder       string
	VcenterResourcePool string
	VcenterCpuNum       int
	VcenterMemSize      int
	VcenterDiskSize     int
	VcenterTemplate     string
	KubernetesVersion   string
}

const (
	// upgrade template
	CAPI_VSPHERE_UPGRADE_TEMPLATE = "capi-vsphere-upgrade-template"

	// machine을 구분하기 위한 label key
	CAPI_CLUSTER_LABEL_KEY      = "cluster.x-k8s.io/cluster-name"
	CAPI_CONTROLPLANE_LABEL_KEY = "cluster.x-k8s.io/control-plane"
	CAPI_WORKER_LABEL_KEY       = "cluster.x-k8s.io/deployment-name"
)
