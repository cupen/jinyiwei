package k8s

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cupen/xdisco/broker"
	"github.com/cupen/xdisco/eventhandler"
	"github.com/cupen/xdisco/server"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"golang.org/x/time/rate"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

// Controller demonstrates how to implement a controller with client-go.
type Controller struct {
	client    *kubernetes.Clientset
	namespace string
	podMeta   *MyPodMeta
	watcher   *cache.ListWatch

	selector fields.Selector
	stateCh  chan server.State
}

func New(selector map[string]string) (*Controller, error) {
	slt := fields.SelectorFromSet(selector)
	c, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	return NewWithConfig(c, slt)
}

func NewWithConfig(c *rest.Config, selector fields.Selector) (*Controller, error) {
	clientset, err := kubernetes.NewForConfig(c)
	if err != nil {
		return nil, err
	}
	cli := clientset.CoreV1().RESTClient()
	podsWatcher := cache.NewListWatchFromClient(cli, "pods", v1.NamespaceDefault, selector)

	return &Controller{
		podMeta:  MustGetMyPodMeta(),
		watcher:  podsWatcher,
		client:   clientset,
		selector: selector,
	}, nil
}

func (c *Controller) show(obj interface{}, event string) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Warn("[k8s] cache.MetaNamespaceKeyFunc failed", zap.Error(err), zap.Reflect("obj", obj))
		return
	}
	p, _ := obj.(*v1.Pod)
	if p == nil {
		log.Warn("[k8s] invalid pod object", zap.Reflect("obj", obj))
		return
	}
	log.Info("[k8s] "+event, zap.String("key", key), zap.Reflect("obj", obj))
}

func (c *Controller) initWatch() {
	if c.stateCh != nil {
		close(c.stateCh)
	}
	c.stateCh = make(chan server.State)
}
func (c *Controller) startWatch(ctx context.Context, h eventhandler.Handler, hc server.Checker, watchCh watch.Interface) {
	limiter := rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
	for {
		select {
		case ev := <-watchCh.ResultChan():
			c.handleEvent(&ev, h, hc)
			// log.Info("watch event", zap.Reflect("ev", ev), zap.Int("count", count))
			limiter.Wait(ctx)
		case <-ctx.Done():
			log2.Infof("[k8s] watch stopped")
			return
		}
	}
}

func (c *Controller) handleEvent(ev *watch.Event, h eventhandler.Handler, hc server.Checker) error {
	if ev == nil {
		return nil
	}
	pod, ok := ev.Object.(*v1.Pod)
	if !ok {
		return fmt.Errorf("skip non-pod")
	}
	if pod.Status.Phase != v1.PodRunning {
		log2.Infof("[k8s] skip pod. %s phase:%v", pod.Name, pod.Status.Phase)
		return nil
	}
	s := podAsServer(pod)
	if s == nil {
		log2.Infof("[k8s] invalid pod. %s annotations:%v ", pod.Name, pod.Annotations)
		return nil
	}
	log2.Infof("[k8s] watch event: %s  pod: %s", ev.Type, pod.Name)
	switch ev.Type {
	case watch.Added:
		h.OnAdd(s.GetKey(), s)
	case watch.Modified:
		h.OnUpdate(s.GetKey(), s)
	case watch.Deleted:
		h.OnDelete(s.GetKey())
	case watch.Bookmark:
		log.Info("[k8s] bookmark", zap.Reflect("object", ev.Object))
	case watch.Error:
		log.Info("[k8s] error", zap.Reflect("object", ev.Object))
	default:
		return nil
	}
	return nil
}

func (c *Controller) startInformer(ctx context.Context, h eventhandler.Handler, hc server.Checker) error {
	watcher := c.watcher
	_, informer := cache.NewIndexerInformer(watcher, &v1.Pod{}, 0, cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.show(obj, "add")
			if p, ok := obj.(*v1.Pod); ok {
				s := podAsServer(p)
				h.OnAdd(s.GetKey(), s)
			} else {
				c.show(obj, "add-debug")
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			c.show(new, "update")
		},
		DeleteFunc: func(obj interface{}) {
			c.show(obj, "delete")
		},
	}, cache.Indexers{})

	localStopCh := make(chan struct{}, 8)
	go informer.Run(localStopCh)

	log.Info("[k8s] watch started.")
	// Wait for all involved caches to be synced, before processing items from the queue is started
	if !cache.WaitForCacheSync(localStopCh, informer.HasSynced) {
		log.Warn("[k8s] watch init failed!!! timed out waiting for sync caches")
		return fmt.Errorf("init failed")
	}
	go func() {
		<-ctx.Done()
		localStopCh <- struct{}{}
	}()
	return nil
}

func (c *Controller) Watch(ctx context.Context, kind string, h eventhandler.Handler, hc server.Checker) error {
	if !h.IsValid() {
		panic(fmt.Errorf("invalid eventhandler"))
	}
	now := time.Now()
	c.initWatch()
	exprs := c.selector.String()
	if exprs != "" {
		exprs += ","
	}
	exprs += "kind=" + kind
	selector, err := labels.Parse(exprs)
	if err != nil {
		return fmt.Errorf("invalid label selector: %s", exprs)
	}
	labelSelector := selector.String()
	log := log.With(zap.String("selector", labelSelector))
	podapi := c.client.CoreV1().Pods("default")
	pods, err := podapi.List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		log.Warn("[k8s] list pods failed", zap.Error(err))
		return err
	}
	servers := []*server.Server{}
	for _, pod := range pods.Items {
		if pod.Status.Phase != v1.PodRunning {
			log2.Infof("[k8s] skip pod. %s phase:%v", pod.Name, pod.Status.Phase)
			continue
		}
		if s := podAsServer(&pod); s != nil {
			servers = append(servers, s)
		}
	}
	h.OnInit(servers)
	log.Info("[k8s] pods found", zap.Int("pods", len(pods.Items)), zap.Int("servers", len(servers)))
	opts := metav1.ListOptions{
		LabelSelector: labelSelector,
		Watch:         true,
	}
	watchCh, err := podapi.Watch(ctx, opts)
	if err != nil {
		log.Warn("[k8s] list pods failed", zap.Error(err))
		return err
	}
	go c.startWatch(ctx, h, hc, watchCh)
	// now := time.Now()
	// if err := c.startInformer(ctx, h, hc); err != nil {
	// 	log.Warn("[k8s] watch failed: start informer failed", zap.Error(err))
	// 	return err
	// }
	log2.Infof("[k8s] watch started. cost: %v", time.Since(now))
	return nil
}

// https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/
func (c *Controller) getNameSpace() string {
	if c.namespace == "" {
		data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
		if err != nil {
			panic(fmt.Errorf("failed to get namespace"))
		}
		np := string(data)
		c.namespace = strings.Trim(np, "\n\t ")
	}
	return c.namespace
}

func (c *Controller) getSelfPod(ctx context.Context) (*v1.Pod, error) {
	meta := c.podMeta
	pod, err := c.client.CoreV1().Pods(meta.Namespace).Get(ctx, meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getSelfPod faild. %w", err)
	}
	return pod, nil
}

// 这个接口可以防止并发修改导致覆盖
// 见官方说明：
// https://github.com/kubernetes/client-go/blob/10e087ca394e2987f09e759438f9949a746c1ca0/examples/create-update-delete-deployment/main.go#L113-L138
func (c *Controller) updateSelfPod(ctx context.Context, updater func(*v1.Pod) error) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		meta := c.podMeta
		podapi := c.client.CoreV1().Pods(meta.Namespace)
		pod, err := podapi.Get(ctx, meta.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get pod failed. %w", err)
		}
		if err := updater(pod); err != nil {
			return fmt.Errorf("change pod failed. %w", err)
		}
		if _, err := podapi.Update(ctx, pod, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("update pod failed. %w", err)
		}
		return nil
	})
	return retryErr
}

func (c *Controller) newServer(kind string) (*server.Server, error) {
	if kind == "" {
		return nil, fmt.Errorf("empty kind")
	}
	pod, err := c.getSelfPod(context.TODO())
	if err != nil {
		return nil, err
	}
	// host := pod.Spec.Containers[0].Name
	// portMeta := pod.Spec.Containers[0].Ports[0]
	port := pod.Spec.Containers[0].Ports[0].ContainerPort
	if port <= 0 {
		return nil, fmt.Errorf("invalid port of container in Pod. podName:%s", pod.ObjectMeta.Name)
	}

	meta := c.podMeta
	// addr := fmt.Sprintf("%s:%d", meta.IP, port)
	// addr := fmt.Sprintf("%s:%d", pod.Name, port)
	id := pod.ObjectMeta.Name
	s := server.NewServer(id, kind, meta.IP)
	s.Labels = pod.GetLabels()
	s.Annotations = pod.GetAnnotations()
	s.Ports = map[string]int{}
	for _, p := range pod.Spec.Containers[0].Ports {
		s.Ports[p.Name] = int(p.ContainerPort)
	}
	return s, nil
}

func (c *Controller) Start(ctx context.Context, s *server.Server, hooks ...broker.Hook) error {
	sn, err := c.newServer(s.Kind)
	if err != nil {
		return err
	}
	sn.Weight = s.Weight
	updater := func(_p *v1.Pod) error {
		return updatePod(_p, sn)
	}
	if err := c.updateSelfPod(ctx, updater); err != nil {
		return err
	}
	if !s.IsValid() {
		return fmt.Errorf("invalid server: %+v", s)
	}
	// FIXME: ...
	*s = *sn
	go func() {
		<-ctx.Done()
		s.Status = string(server.States.Stopping)
		sn.Status = string(server.States.Stopping)
		if err := c.updateSelfPod(ctx, updater); err != nil {
			log.Warn("[k8s] server stopping")
			log.Warn("[k8s] update pod failed", zap.Error(err))
		} else {
			log.Warn("[k8s] server stopping")
		}
	}()
	return nil
}

func (c *Controller) SetState(state server.State) {
}
