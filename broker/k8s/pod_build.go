package k8s

import (
	"fmt"
	"strings"
	"time"

	"github.com/cupen/xdisco/server"
	v1 "k8s.io/api/core/v1"
)

const (
	annotation_keyspace = "xdisco/v1/"
)

func updatePod(pod *v1.Pod, s *server.Server) error {
	port := pod.Spec.Containers[0].Ports[0].ContainerPort
	if port <= 0 {
		return fmt.Errorf("invalid port of container in Pod. podName:%s", pod.ObjectMeta.Name)
	}
	attrs := pod.GetAnnotations()
	if attrs == nil {
		attrs = map[string]string{}
	}
	keyspace := annotation_keyspace
	attrs[keyspace+"kind"] = s.Kind
	attrs[keyspace+"addr"] = s.Addr
	attrs[keyspace+"publicAddr"] = s.PublicAddr
	attrs[keyspace+"status"] = s.Status
	pod.SetAnnotations(attrs)
	return nil
}

func podAsServer(pod *v1.Pod) *server.Server {
	annotations := pod.GetAnnotations()
	annotationsCleaned := map[string]string{}
	for k, v := range annotations {
		if strings.HasPrefix(k, annotation_keyspace) {
			name := strings.TrimPrefix(k, annotation_keyspace)
			annotationsCleaned[name] = v
		}
	}
	s := &server.Server{
		ID:          pod.ObjectMeta.Name,
		Kind:        annotationsCleaned["kind"],
		Addr:        annotationsCleaned["addr"],
		PublicAddr:  annotationsCleaned["publicAddr"],
		Status:      annotationsCleaned["status"],
		Annotations: annotationsCleaned,
		Labels:      pod.GetLabels(),
		CreatedAt:   pod.CreationTimestamp.Time,
		UpdatedAt:   time.Now(),
	}
	if !s.IsValid() {
		return nil
	}
	key := strings.Join([]string{"/k8s/", pod.Namespace, s.Kind, pod.Name}, "/")
	s.SetKey(key)
	return s
}
