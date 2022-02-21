/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmetav1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	clusterv1alpha1 "github.com/tmax-cloud/hypercloud-multi-operator/apis/cluster/v1alpha1"
	util "github.com/tmax-cloud/hypercloud-multi-operator/controllers/util"
	dynamicv2 "github.com/traefik/traefik/v2/pkg/config/dynamic"
	traefikv2 "github.com/traefik/traefik/v2/pkg/provider/kubernetes/crd/traefik/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (r *ClusterManagerReconciler) CreateCertificate(clusterManager *clusterv1alpha1.ClusterManager) error {
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-certificate",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), key, &certmanagerv1.Certificate{}); err != nil {
		if errors.IsNotFound(err) {
			traefikCertificate := &certmanagerv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterManager.Name + "-certificate",
					Namespace: clusterManager.Namespace,
					Annotations: map[string]string{
						util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
						//clusterv1alpha1.AnnotationKeyClmResourceKind: clusterv1alpha1.ClmResourceKindCertificate,
					},
					Labels: map[string]string{
						clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
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
						"multicluster." + clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmDomain],
					},
					IssuerRef: cmmetav1.ObjectReference{
						Name:  "tmaxcloud-issuer",
						Kind:  certmanagerv1.ClusterIssuerKind,
						Group: certmanagerv1.SchemeGroupVersion.Group,
					},
				},
			}
			if err := r.Create(context.TODO(), traefikCertificate); err != nil {
				return err
			}
			ctrl.SetControllerReference(clusterManager, traefikCertificate, r.Scheme)
		}
		return err
	}

	return nil
}

func (r *ClusterManagerReconciler) CreateIngress(clusterManager *clusterv1alpha1.ClusterManager) error {
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-ingress",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), key, &networkingv1.Ingress{}); err != nil {
		if errors.IsNotFound(err) {
			provider := "tmax-cloud"
			pathType := networkingv1.PathTypePrefix
			prefixMiddleware := clusterManager.Namespace + "-" + clusterManager.Name + "-prefix@kubernetescrd"
			multiclusterDNS := "multicluster." + clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmDomain]
			urlPath := "/api/" + clusterManager.Namespace + "/" + clusterManager.Name
			traefikIngress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterManager.Name + "-ingress",
					Namespace: clusterManager.Namespace,
					Annotations: map[string]string{
						util.AnnotationKeyTraefikEntrypoints: "websecure",
						util.AnnotationKeyTraefikMiddlewares: "api-gateway-system-jwt-decode-auth@kubernetescrd," + prefixMiddleware,
						util.AnnotationKeyOwner:              clusterManager.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyCreator:            clusterManager.Annotations[util.AnnotationKeyCreator],
						//clusterv1alpha1.AnnotationKeyClmResourceKind: clusterv1alpha1.ClmResourceKindIngress,
					},
					Labels: map[string]string{
						util.LabelKeyHypercloudIngress:  "multicluster",
						clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
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
											Path:     urlPath,
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: clusterManager.Name + "-service",
													Port: networkingv1.ServiceBackendPort{
														Name: "https",
													},
												},
											},
										},
										{
											Path:     urlPath + "/api/prometheus",
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: clusterManager.Name + "-prometheus-service",
													Port: networkingv1.ServiceBackendPort{
														Number: 443,
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
			if err := r.Create(context.TODO(), traefikIngress); err != nil {
				return err
			}
			ctrl.SetControllerReference(clusterManager, traefikIngress, r.Scheme)
		}
		return err
	}

	return nil
}

func (r *ClusterManagerReconciler) CreateService(clusterManager *clusterv1alpha1.ClusterManager) error {
	if clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver] == "" {
		return fmt.Errorf("ApiServer is not set yet")
	}
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), key, &corev1.Service{}); err != nil {
		if errors.IsNotFound(err) {
			traefikService := &corev1.Service{}
			serviceMeta := &metav1.ObjectMeta{
				Name:      clusterManager.Name + "-service",
				Namespace: clusterManager.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:                  clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator:                clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyTraefikServerTransport: "insecure@file",
					//clusterv1alpha1.AnnotationKeyClmResourceKind: clusterv1alpha1.ClmResourceKindService,
				},
				Labels: map[string]string{
					clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
				},
			}
			if util.IsIpAddress(clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver]) {
				traefikService = &corev1.Service{
					ObjectMeta: *serviceMeta,
					Spec: corev1.ServiceSpec{
						Ports: []corev1.ServicePort{
							{
								Name:       "https",
								Port:       443,
								Protocol:   corev1.ProtocolTCP,
								TargetPort: intstr.FromInt(6443),
							},
						},
					},
				}
			} else {
				traefikService = &corev1.Service{
					ObjectMeta: *serviceMeta,
					Spec: corev1.ServiceSpec{
						ExternalName: clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver],
						Ports: []corev1.ServicePort{
							{
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
			if err := r.Create(context.TODO(), traefikService); err != nil {
				return err
			}
			ctrl.SetControllerReference(clusterManager, traefikService, r.Scheme)
		}
		return err
	}

	return nil
}

func (r *ClusterManagerReconciler) CreateEndpoint(clusterManager *clusterv1alpha1.ClusterManager) error {
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), key, &corev1.Endpoints{}); err != nil {
		if errors.IsNotFound(err) {
			traefikEndpoint := &corev1.Endpoints{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterManager.Name + "-service",
					Namespace: clusterManager.Namespace,
					Annotations: map[string]string{
						util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
						//clusterv1alpha1.AnnotationKeyClmResourceKind: clusterv1alpha1.ClmResourceKindEndpoint,
					},
					Labels: map[string]string{
						clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
					},
				},
				Subsets: []corev1.EndpointSubset{
					{
						Addresses: []corev1.EndpointAddress{
							{
								IP: clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver],
							},
						},
						Ports: []corev1.EndpointPort{
							{
								Name:     "https",
								Port:     6443,
								Protocol: corev1.ProtocolTCP,
							},
						},
					},
				},
			}
			if err := r.Create(context.TODO(), traefikEndpoint); err != nil {
				return err
			}
			ctrl.SetControllerReference(clusterManager, traefikEndpoint, r.Scheme)
		}
		return err
	}

	return nil
}

func CreatePrometheusService(clusterManager *clusterv1alpha1.ClusterManager) *corev1.Service {
	serviceMeta := &metav1.ObjectMeta{
		Name:      clusterManager.Name + "-prometheus-service",
		Namespace: clusterManager.Namespace,
		Annotations: map[string]string{
			util.AnnotationKeyOwner:                  clusterManager.Annotations[util.AnnotationKeyCreator],
			util.AnnotationKeyCreator:                clusterManager.Annotations[util.AnnotationKeyCreator],
			util.AnnotationKeyTraefikServerScheme:    "https",
			util.AnnotationKeyTraefikServerTransport: "insecure@file",
			//clusterv1alpha1.AnnotationKeyClmResourceKind: clusterv1alpha1.ClmResourceKindService,
		},
		Labels: map[string]string{
			clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
		},
	}

	prometheusService := &corev1.Service{}
	if util.IsIpAddress(clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmGateway]) {
		prometheusService = &corev1.Service{
			ObjectMeta: *serviceMeta,
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(443),
					},
				},
			},
		}
	} else {
		prometheusService = &corev1.Service{
			ObjectMeta: *serviceMeta,
			Spec: corev1.ServiceSpec{
				ExternalName: clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmGateway],
				Ports: []corev1.ServicePort{
					{
						Port:       443,
						Protocol:   corev1.ProtocolTCP,
						TargetPort: intstr.FromInt(443),
					},
				},
				Type: corev1.ServiceTypeExternalName,
			},
		}
	}

	return prometheusService
}

func CreatePrometheusEndpoint(clusterManager *clusterv1alpha1.ClusterManager) *corev1.Endpoints {
	prometheusEndpoint := &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterManager.Name + "-prometheus-service",
			Namespace: clusterManager.Namespace,
			Annotations: map[string]string{
				util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
				util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
				//clusterv1alpha1.AnnotationKeyClmResourceKind: clusterv1alpha1.ClmResourceKindEndpoint,
			},
			Labels: map[string]string{
				clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
			},
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmGateway],
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Port:     443,
						Protocol: corev1.ProtocolTCP,
					},
				},
			},
		},
	}

	return prometheusEndpoint
}

func (r *ClusterManagerReconciler) CreateMiddleware(clusterManager *clusterv1alpha1.ClusterManager) error {
	key := types.NamespacedName{
		Name:      clusterManager.Name + "-prefix",
		Namespace: clusterManager.Namespace,
	}
	if err := r.Get(context.TODO(), key, &traefikv2.Middleware{}); err != nil {
		if errors.IsNotFound(err) {
			traefikMiddleware := &traefikv2.Middleware{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterManager.Name + "-prefix",
					Namespace: clusterManager.Namespace,
					Annotations: map[string]string{
						util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
						util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
						//clusterv1alpha1.AnnotationKeyClmResourceKind: clusterv1alpha1.ClmResourceKindMiddleware,
					},
					Labels: map[string]string{
						clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
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
			if err := r.Create(context.TODO(), traefikMiddleware); err != nil {
				return err
			}
			ctrl.SetControllerReference(clusterManager, traefikMiddleware, r.Scheme)
		}
		return err
	}

	return nil
}
