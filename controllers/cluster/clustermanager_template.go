package controllers

import (
	"strings"

	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	tmaxv1 "github.com/tmax-cloud/template-operator/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func buildClusterParams(clm clusterV1alpha1.ClusterManager) []tmaxv1.ParamSpec {
	params := []tmaxv1.ParamSpec{
		buildParam(CLUSTER_PARAM_NAMESPACE, clm.Namespace, intstr.String),
		buildParam(CLUSTER_PARAM_CLUSTER_NAME, clm.Name, intstr.String),
		buildParam(CLUSTER_PARAM_MASTER_NUM, clm.Spec.MasterNum, intstr.Int),
		buildParam(CLUSTER_PARAM_WORKER_NUM, clm.Spec.WorkerNum, intstr.Int),
		buildParam(CLUSTER_PARAM_OWNER, clm.Annotations[util.AnnotationKeyOwner], intstr.String),
		buildParam(CLUSTER_PARAM_KUBERNETES_VERSION, clm.Spec.Version, intstr.String),
	}

	return params
}

func buildAwsParams(spec clusterV1alpha1.ProviderAwsSpec) []tmaxv1.ParamSpec {
	params := []tmaxv1.ParamSpec{
		buildParam(AWS_PARAM_AWS_SSH_KEY_NAME, spec.SshKey, intstr.String),
		buildParam(AWS_PARAM_AWS_REGION, spec.Region, intstr.String),
		buildParam(AWS_PARAM_AWS_CONTROL_PLANE_MACHINE_TYPE, spec.MasterType, intstr.String),
		buildParam(AWS_PARAM_AWS_NODE_MACHINE_TYPE, spec.WorkerType, intstr.String),
		buildParam(AWS_PARAM_MASTER_DISK_SIZE, spec.MasterDiskSize, intstr.Int),
		buildParam(AWS_PARAM_WORKER_DISK_SIZE, spec.WorkerDiskSize, intstr.Int),
	}

	return params
}

func buildVsphereParams(spec clusterV1alpha1.ProviderVsphereSpec) []tmaxv1.ParamSpec {
	params := []tmaxv1.ParamSpec{
		buildParam(VSPHERE_PARAM_POD_CIDR, spec.PodCidr, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_SERVER, spec.VcenterIp, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_USERNAME, spec.VcenterId, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_PASSWORD, spec.VcenterPassword, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_TLS_THUMBPRINT, spec.VcenterThumbprint, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_NETWORK, spec.VcenterNetwork, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_DATACENTER, spec.VcenterDataCenter, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_DATASTORE, spec.VcenterDataStore, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_FOLDER, spec.VcenterFolder, intstr.String),
		buildParam(VSPHERE_PARAM_VSPHERE_RESOURCE_POOL, spec.VcenterResourcePool, intstr.String),
		buildParam(VSPHERE_PARAM_CONTROL_PLANE_ENDPOINT_IP, spec.VcenterKcpIp, intstr.String),
		buildParam(VSPHERE_PARAM_MASTER_CPU_NUM, spec.VcenterCpuNum, intstr.Int),
		buildParam(VSPHERE_PARAM_MASTER_MEM_SIZE, spec.VcenterMemSize, intstr.Int),
		buildParam(VSPHERE_PARAM_MASTER_DISK_SIZE, spec.VcenterDiskSize, intstr.Int),
		buildParam(VSPHERE_PARAM_WORKER_CPU_NUM, spec.VcenterCpuNum, intstr.Int),
		buildParam(VSPHERE_PARAM_WORKER_MEM_SIZE, spec.VcenterMemSize, intstr.Int),
		buildParam(VSPHERE_PARAM_WORKER_DISK_SIZE, spec.VcenterDiskSize, intstr.Int),
		buildParam(VSPHERE_PARAM_VSPHERE_TEMPLATE, spec.VcenterTemplate, intstr.String),
		buildParam(VSPHERE_PARAM_VM_PASSWORD, spec.VMPassword, intstr.String),
	}
	return params
}

func buildVsphereUpgradeParams(clm clusterV1alpha1.ClusterManager) []tmaxv1.ParamSpec {
	params := []tmaxv1.ParamSpec{
		buildParam(VSPHERE_UPGRADE_PARAM_NAMESPACE, clm.Namespace, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_SERVER, clm.VsphereSpec.VcenterIp, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_TLS_THUMBPRINT, clm.VsphereSpec.VcenterThumbprint, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_NETWORK, clm.VsphereSpec.VcenterNetwork, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_DATACENTER, clm.VsphereSpec.VcenterDataCenter, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_DATASTORE, clm.VsphereSpec.VcenterDataStore, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_FOLDER, clm.VsphereSpec.VcenterFolder, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_RESOURCE_POOL, clm.VsphereSpec.VcenterResourcePool, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_TEMPLATE, clm.VsphereSpec.VcenterTemplate, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_CPU_NUM, clm.VsphereSpec.VcenterCpuNum, intstr.Int),
		buildParam(VSPHERE_UPGRADE_PARAM_MEM_SIZE, clm.VsphereSpec.VcenterMemSize, intstr.Int),
		buildParam(VSPHERE_UPGRADE_PARAM_DISK_SIZE, clm.VsphereSpec.VcenterDiskSize, intstr.Int),
		buildParam(VSPHERE_UPGRADE_PARAM_KUBERNETES_VERSION, clm.Spec.Version, intstr.String),
		buildParam(VSPHERE_UPGRADE_PARAM_VSPHERE_TEMPLATE, clm.VsphereSpec.VcenterTemplate, intstr.String),
	}
	return params
}

// ConstructTemplateInstance는 clusterManager를 이용하여 templateInstance를 생성한다.
func ConstructTemplateInstance(clusterManager *clusterV1alpha1.ClusterManager,
	templateInstanceName string,
	parameters []tmaxv1.ParamSpec,
	upgrade bool) (*tmaxv1.TemplateInstance, error) {
	var templateName string

	if upgrade {
		// vsphere upgrade에 대해서만 templateinstance를 생성하므로
		templateName = CAPI_VSPHERE_UPGRADE_TEMPLATE
	} else {
		templateName = "capi-" + strings.ToLower(clusterManager.Spec.Provider) + "-template"
	}

	major, minor, err := ParseK8SVersion(clusterManager)
	if err != nil {
		return nil, err
	}

	// k8s version이 1.19 이하일때는 1.19용 템플릿 사용
	if major == 1 && minor < 20 {
		templateName += "-v1.19"
	}

	annotations := map[string]string{
		util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
		util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
	}

	clusterTemplate := &tmaxv1.ObjectInfo{
		Metadata: tmaxv1.MetadataSpec{
			Name: templateName,
		},
		Parameters: parameters,
	}

	templateInstance := &tmaxv1.TemplateInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        templateInstanceName,
			Namespace:   clusterManager.Namespace,
			Annotations: annotations,
		},
		Spec: tmaxv1.TemplateInstanceSpec{
			ClusterTemplate: clusterTemplate,
		},
	}
	return templateInstance, nil
}
