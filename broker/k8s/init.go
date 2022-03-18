package k8s

import (
	"github.com/cupen/xdisco/logs"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	log  = logs.Logger("info")
	log2 = log.Sugar()
)
var client *kubernetes.Clientset

func IsInK8S() bool {
	cfg, err := rest.InClusterConfig()
	return err == nil && cfg != nil
}

func GetClientInK8S() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	client = clientset
	return client, nil
}
