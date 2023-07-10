package controllers

// for template instance
const (
	// Cluster Parameter
	CLUSTER_PARAM_NAMESPACE          = "NAMESPACE"
	CLUSTER_PARAM_CLUSTER_NAME       = "CLUSTER_NAME"
	CLUSTER_PARAM_MASTER_NUM         = "CONTROL_PLANE_MACHINE_COUNT"
	CLUSTER_PARAM_WORKER_NUM         = "WORKER_MACHINE_COUNT"
	CLUSTER_PARAM_OWNER              = "OWNER"
	CLUSTER_PARAM_KUBERNETES_VERSION = "KUBERNETES_VERSION"

	// Aws Parameter
	AWS_PARAM_AWS_SSH_KEY_NAME               = "AWS_SSH_KEY_NAME"
	AWS_PARAM_AWS_REGION                     = "AWS_REGION"
	AWS_PARAM_AWS_CONTROL_PLANE_MACHINE_TYPE = "AWS_CONTROL_PLANE_MACHINE_TYPE"
	AWS_PARAM_AWS_NODE_MACHINE_TYPE          = "AWS_NODE_MACHINE_TYPE"
	AWS_PARAM_MASTER_DISK_SIZE               = "MASTER_DISK_SIZE"
	AWS_PARAM_WORKER_DISK_SIZE               = "WORKER_DISK_SIZE"

	// Vsphere Parameter
	VSPHERE_PARAM_POD_CIDR                  = "POD_CIDR"
	VSPHERE_PARAM_VSPHERE_SERVER            = "VSPHERE_SERVER"
	VSPHERE_PARAM_VSPHERE_USERNAME          = "VSPHERE_USERNAME"
	VSPHERE_PARAM_VSPHERE_PASSWORD          = "VSPHERE_PASSWORD"
	VSPHERE_PARAM_VSPHERE_TLS_THUMBPRINT    = "VSPHERE_TLS_THUMBPRINT"
	VSPHERE_PARAM_VSPHERE_NETWORK           = "VSPHERE_NETWORK"
	VSPHERE_PARAM_VSPHERE_DATACENTER        = "VSPHERE_DATACENTER"
	VSPHERE_PARAM_VSPHERE_DATASTORE         = "VSPHERE_DATASTORE"
	VSPHERE_PARAM_VSPHERE_FOLDER            = "VSPHERE_FOLDER"
	VSPHERE_PARAM_VSPHERE_RESOURCE_POOL     = "VSPHERE_RESOURCE_POOL"
	VSPHERE_PARAM_MASTER_CPU_NUM            = "MASTER_CPU_NUM"
	VSPHERE_PARAM_MASTER_MEM_SIZE           = "MASTER_MEM_SIZE"
	VSPHERE_PARAM_MASTER_DISK_SIZE          = "MASTER_DISK_SIZE"
	VSPHERE_PARAM_WORKER_CPU_NUM            = "WORKER_CPU_NUM"
	VSPHERE_PARAM_WORKER_MEM_SIZE           = "WORKER_MEM_SIZE"
	VSPHERE_PARAM_WORKER_DISK_SIZE          = "WORKER_DISK_SIZE"
	VSPHERE_PARAM_VSPHERE_TEMPLATE          = "VSPHERE_TEMPLATE"
	VSPHERE_PARAM_VM_PASSWORD               = "VM_PASSWORD"
	VSPHERE_PARAM_CONTROL_PLANE_ENDPOINT_IP = "CONTROL_PLANE_ENDPOINT_IP"

	// Vsphere UpgradeParameter
	VSPHERE_UPGRADE_PARAM_NAMESPACE              = "NAMESPACE"
	VSPHERE_UPGRADE_PARAM_UPGRADE_TEMPLATE_NAME  = "UPGRADE_TEMPLATE_NAME"
	VSPHERE_UPGRADE_PARAM_VSPHERE_SERVER         = "VSPHERE_SERVER"
	VSPHERE_UPGRADE_PARAM_VSPHERE_TLS_THUMBPRINT = "VSPHERE_TLS_THUMBPRINT"
	VSPHERE_UPGRADE_PARAM_VSPHERE_NETWORK        = "VSPHERE_NETWORK"
	VSPHERE_UPGRADE_PARAM_VSPHERE_DATACENTER     = "VSPHERE_DATACENTER"
	VSPHERE_UPGRADE_PARAM_VSPHERE_DATASTORE      = "VSPHERE_DATASTORE"
	VSPHERE_UPGRADE_PARAM_VSPHERE_FOLDER         = "VSPHERE_FOLDER"
	VSPHERE_UPGRADE_PARAM_VSPHERE_RESOURCE_POOL  = "VSPHERE_RESOURCE_POOL"
	VSPHERE_UPGRADE_PARAM_CPU_NUM                = "CPU_NUM"
	VSPHERE_UPGRADE_PARAM_MEM_SIZE               = "MEM_SIZE"
	VSPHERE_UPGRADE_PARAM_DISK_SIZE              = "DISK_SIZE"
	VSPHERE_UPGRADE_PARAM_VSPHERE_TEMPLATE       = "VSPHERE_TEMPLATE"
	VSPHERE_UPGRADE_PARAM_KUBERNETES_VERSION     = "KUBERNETES_VERSION"
)

const (
	LabelKeyControlplaneNode   = "node-role.kubernetes.io/master"
	LabelValueControlplaneNode = ""
)

const (
	LabelKeyCAPIClusterName = "cluster.x-k8s.io/cluster-name"
)