package util

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"net/url"
	"strings"
	"time"

	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	dynamicv2 "github.com/traefik/traefik/v2/pkg/config/dynamic"
	traefikv2 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
)

// LowestNonZeroResult compares two reconciliation results
// and returns the one with lowest requeue time.
func LowestNonZeroResult(i, j ctrl.Result) ctrl.Result {
	switch {
	case i.IsZero():
		return j
	case j.IsZero():
		return i
	case i.Requeue:
		return i
	case j.Requeue:
		return j
	case i.RequeueAfter < j.RequeueAfter:
		return i
	default:
		return j
	}
}

// func Goid() int {
// 	var buf [64]byte
// 	n := runtime.Stack(buf[:], false)
// 	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
// 	id, err := strconv.Atoi(idField)
// 	if err != nil {
// 		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
// 	}
// 	return id
// }

func GetRemoteK8sClient(secret *corev1.Secret) (*kubernetes.Clientset, error) {
	var remoteClientset *kubernetes.Clientset
	if value, ok := secret.Data["value"]; ok {
		remoteClientConfig, err := clientcmd.NewClientConfigFromBytes(value)
		if err != nil {
			return nil, err
		}
		remoteRestConfig, err := remoteClientConfig.ClientConfig()
		if err != nil {
			return nil, err
		}
		remoteClientset, err = kubernetes.NewForConfig(remoteRestConfig)
		if err != nil {
			return nil, err
		}
	} else {
		err := errors.NewBadRequest("secret does not have a value")
		return nil, err
	}
	return remoteClientset, nil
}

func GetRemoteK8sClientByKubeConfig(kubeConfig []byte) (*kubernetes.Clientset, error) {
	var remoteClientset *kubernetes.Clientset

	remoteClientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	remoteRestConfig, err := remoteClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	remoteClientset, err = kubernetes.NewForConfig(remoteRestConfig)
	if err != nil {
		return nil, err
	}

	return remoteClientset, nil
}

func GetK8sClient() (*kubernetes.Clientset, error) {
	// var Clientset *kubernetes.Clientset
	var err error

	config, err := restclient.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	config.Burst = 100
	config.QPS = 100
	Clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return Clientset, nil
}

func CreateSuffixString() string {
	rand.Seed(time.Now().UnixNano())
	var letters = []rune("0123456789abcdefghijklmnopqrstuvwxyz")

	s := make([]rune, SuffixDigit)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}

	return string(s)
}

func CreateCertificate(clusterManager *clusterv1alpha1.ClusterManager) *certmanagerv1.Certificate {
	traefikCertificate := &certmanagerv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			//Name:      clusterManager.Name + "-certificate" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
			Name:      clusterManager.Name + "-certificate",
			Namespace: clusterManager.Namespace,
			Annotations: map[string]string{
				AnnotationKeyOwner:   clusterManager.Annotations[AnnotationKeyCreator],
				AnnotationKeyCreator: clusterManager.Annotations[AnnotationKeyCreator],
			},
			Labels: map[string]string{
				LabelKeyClmRef: clusterManager.Name,
			},
		},
		Spec: certmanagerv1.CertificateSpec{
			SecretName: clusterManager.Name + "-service-cert",
			IsCA:       false,
			Usages: []certmanagerv1.KeyUsage{
				certmanagerv1.UsageDigitalSignature,
				certmanagerv1.UsageKeyEncipherment,
				certmanagerv1.UsageServerAuth,
				certmanagerv1.UsageClientAuth,
			},
			DNSNames: []string{
				"multicluster." + clusterManager.Annotations[AnnotationKeyClmDns],
			},
			IssuerRef: cmmetav1.ObjectReference{
				Name:  "tmaxcloud-issuer",
				Kind:  "ClusterIssuer",
				Group: "cert-manager.io",
			},
		},
	}

	return traefikCertificate
}

func CreateIngress(clusterManager *clusterv1alpha1.ClusterManager) *networkingv1.Ingress {
	provider := HypercloudIngressClass
	pathType := networkingv1.PathTypePrefix
	prefixMiddleware := clusterManager.Namespace + "-" + clusterManager.Name + "-prefix@kubernetescrd"
	multiclusterDNS := "multicluster." + clusterManager.Annotations[AnnotationKeyClmDns]
	traefikIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			//Name:      clusterManager.Name + "-ingress-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
			Name:      clusterManager.Name + "-ingress",
			Namespace: clusterManager.Namespace,
			Annotations: map[string]string{
				AnnotationKeyTraefikEntrypoints: "websecure",
				AnnotationKeyTraefikMiddlewares: "api-gateway-system-jwt-decode-auth@kubernetescrd," + prefixMiddleware,
				AnnotationKeyOwner:              clusterManager.Annotations[AnnotationKeyCreator],
				AnnotationKeyCreator:            clusterManager.Annotations[AnnotationKeyCreator],
			},
			Labels: map[string]string{
				LabelKeyHypercloudIngress: HypercloudMultiIngressClass,
				LabelKeyClmRef:            clusterManager.GetName(),
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &provider,
			Rules: []networkingv1.IngressRule{
				{
					Host: multiclusterDNS,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/api/" + clusterManager.Namespace + "/" + clusterManager.Name,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: clusterManager.GetName() + "-service",
											Port: networkingv1.ServiceBackendPort{
												//Name: strings.ToLower(string(corev1.URISchemeHTTPS)),
												Name: "https",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{
						multiclusterDNS,
					},
				},
			},
		},
	}

	return traefikIngress
}

func CreateService(clusterManager *clusterv1alpha1.ClusterManager) *corev1.Service {
	serviceMeta := &metav1.ObjectMeta{
		//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
		Annotations: map[string]string{
			AnnotationKeyOwner:                  clusterManager.Annotations[AnnotationKeyCreator],
			AnnotationKeyCreator:                clusterManager.Annotations[AnnotationKeyCreator],
			AnnotationKeyTraefikServerTransport: "insecure@file",
		},
		Labels: map[string]string{
			LabelKeyClmRef: clusterManager.Name,
		},
	}
	traefikService := &corev1.Service{}
	switch strings.ToUpper(clusterManager.Spec.Provider) {
	case ProviderAws:
		traefikService = &corev1.Service{
			ObjectMeta: *serviceMeta,
			Spec: corev1.ServiceSpec{
				ExternalName: clusterManager.Annotations[AnnotationKeyClmEndpoint],
				Ports: []corev1.ServicePort{
					{
						//Name:       strings.ToLower(string(corev1.URISchemeHTTPS)),
						Name:       "https",
						Port:       6443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(6443),
					},
				},
				Type: corev1.ServiceTypeExternalName,
			},
		}
	case ProviderVsphere:
		traefikService = &corev1.Service{
			ObjectMeta: *serviceMeta,
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						//Name:       strings.ToLower(string(corev1.URISchemeHTTPS)),
						Name:       "https",
						Port:       443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(6443),
					},
				},
			},
		}
	default:
		traefikService = &corev1.Service{
			ObjectMeta: *serviceMeta,
			Spec: corev1.ServiceSpec{
				ExternalName: clusterManager.Annotations[AnnotationKeyClmEndpoint],
				Ports: []corev1.ServicePort{
					{
						//Name:       strings.ToLower(string(corev1.URISchemeHTTPS)),
						Name:       "https",
						Port:       6443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(6443),
					},
				},
				Type: corev1.ServiceTypeExternalName,
			},
		}
	}

	return traefikService
}

func CreateEndpoint(clusterManager *clusterv1alpha1.ClusterManager) *corev1.Endpoints {
	traefikEndpoint := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			//Name:      clusterManager.Name + "-service-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
			Name:      clusterManager.Name + "-service",
			Namespace: clusterManager.Namespace,
			Annotations: map[string]string{
				AnnotationKeyOwner:   clusterManager.Annotations[AnnotationKeyCreator],
				AnnotationKeyCreator: clusterManager.Annotations[AnnotationKeyCreator],
			},
			Labels: map[string]string{
				LabelKeyClmRef: clusterManager.Name,
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: clusterManager.Annotations[AnnotationKeyClmEndpoint],
					},
				},
				Ports: []corev1.EndpointPort{
					{
						//Name: strings.ToLower(string(corev1.URISchemeHTTPS)),
						Name: "https",
						Port: 6443,
					},
				},
			},
		},
	}

	return traefikEndpoint
}

func CreateMiddleware(clusterManager *clusterv1alpha1.ClusterManager) *traefikv2.Middleware {
	traefikMiddleware := &traefikv2.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			//Name:      clusterManager.Name + "-prefix-" + clusterManager.Annotations[util.AnnotationKeyClmSuffix],
			Name:      clusterManager.Name + "-prefix",
			Namespace: clusterManager.Namespace,
			Annotations: map[string]string{
				AnnotationKeyOwner:   clusterManager.Annotations[AnnotationKeyCreator],
				AnnotationKeyCreator: clusterManager.Annotations[AnnotationKeyCreator],
			},
			Labels: map[string]string{
				LabelKeyClmRef: clusterManager.Name,
			},
		},
		Spec: traefikv2.MiddlewareSpec{
			StripPrefix: &dynamicv2.StripPrefix{
				Prefixes: []string{
					"/api/" + clusterManager.Namespace + "/" + clusterManager.Name,
				},
			},
		},
	}
	return traefikMiddleware
}

func MergeJson(dest []byte, source []byte) []byte {
	dest = append(dest[0:len(dest)-1], 44)
	dest = append(dest, source[1:]...)
	return dest
}

func URIToSecretName(uriType, uri string) (string, error) {
	parsedURI, err := url.ParseRequestURI(uri)
	if err != nil {
		return "", err
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(uri))
	host := strings.ToLower(strings.Split(parsedURI.Host, ":")[0])
	return fmt.Sprintf("%s-%s-%v", uriType, host, h.Sum32()), nil
}

func GetProviderName(provider string) (string, error) {
	provider = strings.ToUpper(provider)
	providerNamelogo := map[string]string{
		ProviderAws:     "AWS",
		ProviderVsphere: "vSphere",
	}

	if providerNamelogo[provider] == "" {
		return ProviderUnknown, fmt.Errorf("Cannot found provider [" + provider + "]")
	}

	return providerNamelogo[provider], nil
}
