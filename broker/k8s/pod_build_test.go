package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
)

func TestPod(t *testing.T) {
	assert := assert.New(t)
	assert.Equal(0, 1)

	pod := &v1.Pod{}
	pod.SetAnnotations(map[string]string{
		"xdisco/v1/addr":   "127.0.0.1:11",
		"xdisco/v1/kind":   "test",
		"xdisco/v1/status": "yes",
	})
	s := podAsServer(pod)
	assert.Equal(s.Addr, "127.0.0.1:11")
	assert.Equal(s.Kind, "test")
	assert.Equal(s.Status, "yes")

	assert.Equal(s.Annotations["kind"], "test")
	assert.Equal(s.Annotations["status"], "yes")
	assert.Equal(s.Annotations["addr"], "127.0.0.1:11")
}
