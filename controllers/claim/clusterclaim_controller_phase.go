package controllers

import (
	"context"
	"fmt"
	"os"
	"strings"

	claimV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/claim/v1alpha1"
	clusterV1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	"github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ClusterClaimReconciler) CreateClusterManager(ctx context.Context, cc *claimV1alpha1.ClusterClaim) error {

	clmKey := cc.GetClusterManagerNamespacedName()
	clm := &clusterV1alpha1.ClusterManager{}

	if err := r.Client.Get(context.TODO(), clmKey, clm); errors.IsNotFound(err) {
		clm, err := r.ConstructClusterManagerByClaim(cc)
		if err != nil {
			return err
		}
		if err := r.Create(context.TODO(), &clm); err != nil {
			return err
		}

	} else if err != nil {
		return err
	}

	return nil
}

func (r *ClusterClaimReconciler) ConstructClusterManagerByClaim(cc *claimV1alpha1.ClusterClaim) (clusterV1alpha1.ClusterManager, error) {
	clmSpec := clusterV1alpha1.ClusterManagerSpec{
		Provider:  cc.Spec.Provider,
		Version:   cc.Spec.Version,
		MasterNum: cc.Spec.MasterNum,
		WorkerNum: cc.Spec.WorkerNum,
	}

	clm := clusterV1alpha1.ClusterManager{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      cc.Spec.ClusterName,
			Namespace: cc.Namespace,
			Labels: map[string]string{
				clusterV1alpha1.LabelKeyClmClusterType: clusterV1alpha1.ClusterTypeCreated,
				clusterV1alpha1.LabelKeyClcName:        cc.Name,
			},
			Annotations: map[string]string{
				"owner":                                cc.Annotations[util.AnnotationKeyCreator],
				"creator":                              cc.Annotations[util.AnnotationKeyCreator],
				clusterV1alpha1.AnnotationKeyClmDomain: os.Getenv(util.HC_DOMAIN),
			},
		},
		Spec: clmSpec,
	}

	if util.IsAWSProvider(cc.Spec.Provider) {
		awsSpec, err := NewAwsSpec(cc)
		if err != nil {
			return clusterV1alpha1.ClusterManager{}, err
		}
		clm.AwsSpec = awsSpec
	} else if util.IsVsphereProvider(cc.Spec.Provider) {
		vsphereSpec, err := NewVsphereSpec(cc)
		if err != nil {
			return clusterV1alpha1.ClusterManager{}, err
		}

		clm.VsphereSpec = vsphereSpec
		if err := r.LoadVsphereCredentials(&clm); err != nil {
			return clusterV1alpha1.ClusterManager{}, err
		}
	}

	return clm, nil
}

// vsphere spec configuration
func NewVsphereSpec(cc *claimV1alpha1.ClusterClaim) (clusterV1alpha1.ProviderVsphereSpec, error) {

	podCidr := cc.Spec.ProviderVsphereSpec.PodCidr
	if podCidr == "" {
		podCidr = "10.0.0.0/16"
	}

	thumbPrint, err := util.AddColonToThumbprint(cc.Spec.ProviderVsphereSpec.VcenterThumbprint)
	if err != nil {
		return clusterV1alpha1.ProviderVsphereSpec{}, err
	}

	vmNetwork := cc.Spec.ProviderVsphereSpec.VcenterNetwork
	if vmNetwork == "" {
		vmNetwork = "VM Network"
	}

	vcenterFolder := cc.Spec.ProviderVsphereSpec.VcenterFolder
	if vcenterFolder == "" {
		vcenterFolder = "vm"
	}

	cpu := cc.Spec.ProviderVsphereSpec.VcenterCpuNum
	if cpu == 0 {
		cpu = 2
	}

	mem := cc.Spec.ProviderVsphereSpec.VcenterMemSize
	if mem == 0 {
		mem = 4096
	}

	diskSize := cc.Spec.ProviderVsphereSpec.VcenterDiskSize
	if diskSize == 0 {
		diskSize = 20
	}

	vmPassword := cc.Spec.ProviderVsphereSpec.VMPassword
	if vmPassword == "" {
		vmPassword = "dG1heEAyMw=="
	}

	return clusterV1alpha1.ProviderVsphereSpec{
		PodCidr:             podCidr,
		VcenterCpuNum:       cpu,
		VcenterMemSize:      mem,
		VcenterDiskSize:     diskSize,
		VcenterThumbprint:   thumbPrint,
		VcenterNetwork:      vmNetwork,
		VcenterFolder:       vcenterFolder,
		VMPassword:          vmPassword,
		VcenterIp:           cc.Spec.ProviderVsphereSpec.VcenterIp,
		VcenterDataCenter:   cc.Spec.ProviderVsphereSpec.VcenterDataCenter,
		VcenterDataStore:    cc.Spec.ProviderVsphereSpec.VcenterDataStore,
		VcenterResourcePool: cc.Spec.ProviderVsphereSpec.VcenterResourcePool,
		VcenterKcpIp:        cc.Spec.ProviderVsphereSpec.VcenterKcpIp,
		VcenterTemplate:     cc.Spec.ProviderVsphereSpec.VcenterTemplate,
	}, nil
}

// aws spec configuration
func NewAwsSpec(cc *claimV1alpha1.ClusterClaim) (clusterV1alpha1.ProviderAwsSpec, error) {
	region := cc.Spec.ProviderAwsSpec.Region
	if region == "" {
		region = "ap-northeast-2"
	}

	masterType := cc.Spec.ProviderAwsSpec.MasterType
	if masterType == "" {
		masterType = "t3.medium"
	}

	workerType := cc.Spec.ProviderAwsSpec.WorkerType
	if workerType == "" {
		workerType = "t3.medium"
	}

	masterDiskSize := cc.Spec.ProviderAwsSpec.MasterDiskSize
	if masterDiskSize == 0 {
		masterDiskSize = 20
	}

	workerDiskSize := cc.Spec.ProviderAwsSpec.WorkerDiskSize
	if workerDiskSize == 0 {
		workerDiskSize = 20
	}

	return clusterV1alpha1.ProviderAwsSpec{
		Region:         region,
		MasterType:     masterType,
		MasterDiskSize: masterDiskSize,
		WorkerType:     workerType,
		WorkerDiskSize: workerDiskSize,
		SshKey:         cc.Spec.ProviderAwsSpec.SshKey,
	}, nil
}

func (r *ClusterClaimReconciler) LoadVsphereCredentials(clm *clusterV1alpha1.ClusterManager) error {

	key := types.NamespacedName{
		Name:      "capv-manager-bootstrap-credentials",
		Namespace: "capv-system",
	}

	credential := &coreV1.Secret{}

	if err := r.Client.Get(context.TODO(), key, credential); err != nil {
		return err
	}

	credentials, ok := credential.Data["credentials.yaml"]
	if !ok {
		return fmt.Errorf("credentials info not found in vsphere credential secret")
	}

	lines := strings.Split(string(credentials), "\n")
	username := strings.Split(lines[0], ":")[1]
	password := strings.Split(lines[1], ":")[1]

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" || password == "" {
		return fmt.Errorf("username or password not found in vsphere credential secret")
	}

	clm.VsphereSpec.VcenterId = username
	clm.VsphereSpec.VcenterPassword = password
	return nil
}
