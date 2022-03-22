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
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-certificate",
		Namespace: clusterManager.Namespace,
	}
	err := r.Get(context.TODO(), key, &certmanagerv1.Certificate{})
	if errors.IsNotFound(err) {
		certificate := &certmanagerv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterManager.Name + "-certificate",
				Namespace: clusterManager.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
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
		if err := r.Create(context.TODO(), certificate); err != nil {
			log.Error(err, "Failed to Create Certificate")
			return err
		}

		log.Info("Create Certificate successfully")
		ctrl.SetControllerReference(clusterManager, certificate, r.Scheme)
		return nil
	}

	return err
}

func (r *ClusterManagerReconciler) CreateIngress(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-ingress",
		Namespace: clusterManager.Namespace,
	}
	err := r.Get(context.TODO(), key, &networkingv1.Ingress{})
	if errors.IsNotFound(err) {
		provider := "tmax-cloud"
		pathType := networkingv1.PathTypePrefix
		prefixMiddleware := clusterManager.Namespace + "-" + clusterManager.Name + "-prefix@kubernetescrd"
		multiclusterDNS := "multicluster." + clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmDomain]
		urlPath := "/api/" + clusterManager.Namespace + "/" + clusterManager.Name
		ingress := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterManager.Name + "-ingress",
				Namespace: clusterManager.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyTraefikEntrypoints: "websecure",
					util.AnnotationKeyTraefikMiddlewares: "api-gateway-system-jwt-decode-auth@kubernetescrd," + prefixMiddleware,
					util.AnnotationKeyOwner:              clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator:            clusterManager.Annotations[util.AnnotationKeyCreator],
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
												Name: clusterManager.Name + "-gateway-service",
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
		if err := r.Create(context.TODO(), ingress); err != nil {
			log.Error(err, "Failed to Create Ingress")
			return err
		}

		log.Info("Create Ingress successfully")
		ctrl.SetControllerReference(clusterManager, ingress, r.Scheme)
		return nil
	}

	return err
}

func (r *ClusterManagerReconciler) CreateService(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	if clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver] == "" {
		return fmt.Errorf("ApiServer is not set yet")
	}

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	err := r.Get(context.TODO(), key, &corev1.Service{})
	if errors.IsNotFound(err) {
		service := &corev1.Service{}
		metadata := &metav1.ObjectMeta{
			Name:      clusterManager.Name + "-service",
			Namespace: clusterManager.Namespace,
			Annotations: map[string]string{
				util.AnnotationKeyOwner:                  clusterManager.Annotations[util.AnnotationKeyCreator],
				util.AnnotationKeyCreator:                clusterManager.Annotations[util.AnnotationKeyCreator],
				util.AnnotationKeyTraefikServerTransport: "insecure@file",
			},
			Labels: map[string]string{
				clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
			},
		}
		if util.IsIpAddress(clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmApiserver]) {
			service = &corev1.Service{
				ObjectMeta: *metadata,
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
			service = &corev1.Service{
				ObjectMeta: *metadata,
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
		if err := r.Create(context.TODO(), service); err != nil {
			log.Error(err, "Failed to Create Service")
			return err
		}

		log.Info("Create Service successfully")
		ctrl.SetControllerReference(clusterManager, service, r.Scheme)
		return nil
	}

	return err
}

func (r *ClusterManagerReconciler) CreateEndpoint(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	err := r.Get(context.TODO(), key, &corev1.Endpoints{})
	if errors.IsNotFound(err) {
		endpoint := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterManager.Name + "-service",
				Namespace: clusterManager.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
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
		if err := r.Create(context.TODO(), endpoint); err != nil {
			log.Error(err, "Failed to Create Endpoint")
			return err
		}

		log.Info("Create Endpoint successfully")
		ctrl.SetControllerReference(clusterManager, endpoint, r.Scheme)
		return nil
	}

	return err
}

func (r *ClusterManagerReconciler) CreateGatewayService(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-gateway-service",
		Namespace: clusterManager.Namespace,
	}
	err := r.Get(context.TODO(), key, &corev1.Service{})
	if errors.IsNotFound(err) {
		service := &corev1.Service{}
		metadata := &metav1.ObjectMeta{
			Name:      clusterManager.Name + "-gateway-service",
			Namespace: clusterManager.Namespace,
			Annotations: map[string]string{
				util.AnnotationKeyOwner:                  clusterManager.Annotations[util.AnnotationKeyCreator],
				util.AnnotationKeyCreator:                clusterManager.Annotations[util.AnnotationKeyCreator],
				util.AnnotationKeyTraefikServerScheme:    "https",
				util.AnnotationKeyTraefikServerTransport: "insecure@file",
			},
			Labels: map[string]string{
				clusterv1alpha1.LabelKeyClmName: clusterManager.Name,
			},
		}
		if util.IsIpAddress(clusterManager.Annotations[clusterv1alpha1.AnnotationKeyClmGateway]) {
			service = &corev1.Service{
				ObjectMeta: *metadata,
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
			service = &corev1.Service{
				ObjectMeta: *metadata,
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
		if err := r.Create(context.TODO(), service); err != nil {
			log.Error(err, "Failed to Create Service for gateway")
			return err
		}

		log.Info("Create Service for gateway successfully")
		ctrl.SetControllerReference(clusterManager, service, r.Scheme)
		return nil
	}

	return err
}

func (r *ClusterManagerReconciler) CreateGatewayEndpoint(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-gateway-service",
		Namespace: clusterManager.Namespace,
	}
	err := r.Get(context.TODO(), key, &corev1.Endpoints{})
	if errors.IsNotFound(err) {
		endpoint := &corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterManager.Name + "-gateway-service",
				Namespace: clusterManager.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
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
		if err := r.Create(context.TODO(), endpoint); err != nil {
			log.Error(err, "Failed to Create Endpoint for gateway")
			return err
		}

		log.Info("Create Endpoint for gateway successfully")
		ctrl.SetControllerReference(clusterManager, endpoint, r.Scheme)
		return nil
	}

	return err
}

func (r *ClusterManagerReconciler) CreateMiddleware(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-prefix",
		Namespace: clusterManager.Namespace,
	}
	err := r.Get(context.TODO(), key, &traefikv2.Middleware{})
	if errors.IsNotFound(err) {
		middleware := &traefikv2.Middleware{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterManager.Name + "-prefix",
				Namespace: clusterManager.Namespace,
				Annotations: map[string]string{
					util.AnnotationKeyOwner:   clusterManager.Annotations[util.AnnotationKeyCreator],
					util.AnnotationKeyCreator: clusterManager.Annotations[util.AnnotationKeyCreator],
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
		if err := r.Create(context.TODO(), middleware); err != nil {
			log.Error(err, "Failed to Create Middleware")
			return err
		}

		log.Info("Create Middleware successfully")
		ctrl.SetControllerReference(clusterManager, middleware, r.Scheme)
		return nil
	}

	return err
}

func (r *ClusterManagerReconciler) DeleteCertificate(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-certificate",
		Namespace: clusterManager.Namespace,
	}
	certificate := &certmanagerv1.Certificate{}
	err := r.Get(context.TODO(), key, certificate)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Certificate")
		return err
	}

	if err := r.Delete(context.TODO(), certificate); err != nil {
		log.Error(err, "Failed to delete Certificate")
		return err
	}

	log.Info("Delete Certificate successfully")
	return nil
}

func (r *ClusterManagerReconciler) DeleteCertSecret(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-service-cert",
		Namespace: clusterManager.Namespace,
	}
	secret := &corev1.Secret{}
	err := r.Get(context.TODO(), key, secret)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Secret for certificate")
		return err
	}

	if err := r.Delete(context.TODO(), secret); err != nil {
		log.Error(err, "Failed to delete Secret for certificate")
		return err
	}

	log.Info("Delete Secret for certificate successfully")
	return nil
}

func (r *ClusterManagerReconciler) DeleteIngress(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-ingress",
		Namespace: clusterManager.Namespace,
	}
	ingress := &networkingv1.Ingress{}
	err := r.Get(context.TODO(), key, ingress)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Ingress")
		return err
	}

	if err := r.Delete(context.TODO(), ingress); err != nil {
		log.Error(err, "Failed to delete Ingress")
		return err
	}

	log.Info("Delete Ingress successfully")
	return nil
}

func (r *ClusterManagerReconciler) DeleteService(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	service := &corev1.Service{}
	err := r.Get(context.TODO(), key, service)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Service")
		return err
	}

	if err := r.Delete(context.TODO(), service); err != nil {
		log.Error(err, "Failed to delete Service")
		return err
	}

	log.Info("Delete Service successfully")
	return nil
}

func (r *ClusterManagerReconciler) DeleteEndpoint(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-service",
		Namespace: clusterManager.Namespace,
	}
	endpoint := &corev1.Endpoints{}
	err := r.Get(context.TODO(), key, endpoint)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Endpoint")
		return err
	}

	if err := r.Delete(context.TODO(), endpoint); err != nil {
		log.Error(err, "Failed to delete Endpoint")
		return err
	}

	log.Info("Delete Endpoint successfully")
	return nil
}

func (r *ClusterManagerReconciler) DeleteMiddleware(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-prefix",
		Namespace: clusterManager.Namespace,
	}
	middleware := &traefikv2.Middleware{}
	err := r.Get(context.TODO(), key, middleware)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Middleware")
		return err
	}

	if err := r.Delete(context.TODO(), middleware); err != nil {
		log.Error(err, "Failed to delete Middleware")
		return err
	}

	log.Info("Delete Middleware successfully")
	return nil
}

func (r *ClusterManagerReconciler) DeleteGatewayService(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-gateway-service",
		Namespace: clusterManager.Namespace,
	}
	service := &corev1.Service{}
	err := r.Get(context.TODO(), key, service)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Service")
		return err
	}

	if err := r.Delete(context.TODO(), service); err != nil {
		log.Error(err, "Failed to delete Service")
		return err
	}

	log.Info("Delete Service successfully")
	return nil
}

func (r *ClusterManagerReconciler) DeleteGatewayEndpoint(clusterManager *clusterv1alpha1.ClusterManager) error {
	log := r.Log.WithValues("clustermanager", clusterManager.GetNamespacedName())

	key := types.NamespacedName{
		Name:      clusterManager.Name + "-gateway-service",
		Namespace: clusterManager.Namespace,
	}
	endpoint := &corev1.Endpoints{}
	err := r.Get(context.TODO(), key, endpoint)
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		log.Error(err, "Failed to get Endpoint")
		return err
	}

	if err := r.Delete(context.TODO(), endpoint); err != nil {
		log.Error(err, "Failed to delete Endpoint")
		return err
	}

	log.Info("Delete Endpoint successfully")
	return nil
}
