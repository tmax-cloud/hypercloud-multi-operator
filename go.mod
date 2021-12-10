module github.com/tmax-cloud/hypercloud-multi-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/jetstack/cert-manager v1.5.4
	github.com/kubernetes-sigs/service-catalog v0.3.1
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/tmax-cloud/console-operator v0.0.0-20210202020310-14940831c3ba
	github.com/traefik/traefik/v2 v2.5.4
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	sigs.k8s.io/cluster-api v0.4.4
	sigs.k8s.io/controller-runtime v0.10.1
	//	sigs.k8s.io/kubefed v0.8.1
	sigs.k8s.io/yaml v1.2.0
)

// for traefik
replace (
	github.com/abbot/go-http-auth => github.com/containous/go-http-auth v0.4.1-0.20200324110947-a37a7636d23e
	github.com/go-check/check => github.com/containous/check v0.0.0-20170915194414-ca0bf163426a
	github.com/gorilla/mux => github.com/containous/mux v0.0.0-20181024131434-c33f32e26898
	github.com/mailgun/minheap => github.com/containous/minheap v0.0.0-20190809180810-6e71eb837595
	github.com/mailgun/multibuf => github.com/containous/multibuf v0.0.0-20190809014333-8b6c9a7e6bba
)
