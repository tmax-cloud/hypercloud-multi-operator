package util

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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

func Goid() int {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

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
