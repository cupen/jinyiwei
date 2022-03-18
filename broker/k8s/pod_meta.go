package k8s

import (
	"fmt"
	"os"
)

type MyPodMeta struct {
	Namespace string
	Name      string
	IP        string
}

func getenv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("missing environment var:%s", key)
	}
	return value, nil
}

func MustGetMyPodMeta() *MyPodMeta {
	name, err := getenv("MY_POD_NAME")
	if err != nil {
		panic(err)
	}
	namespace, err := getenv("MY_POD_NAMESPACE")
	if err != nil {
		panic(err)
	}
	ip, err := getenv("MY_POD_IP")
	if err != nil {
		panic(err)
	}
	return &MyPodMeta{
		Name:      name,
		Namespace: namespace,
		IP:        ip,
	}
}
