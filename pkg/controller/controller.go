package controller

import (
	"crypto/sha256"
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/uzxmx/kubenotify/pkg/config"
	"github.com/uzxmx/kubenotify/pkg/handlers"
	"github.com/uzxmx/kubenotify/pkg/utils"
	apps_v1 "k8s.io/api/apps/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type Controller struct {
	kubeClient kubernetes.Interface
	handler    handlers.Handler
	queue      workqueue.RateLimitingInterface
	targets    map[string]*Target
}

type Event struct {
	obj       interface{}
	eventType string
}

type Target struct {
	obj                 interface{}
	lastTimeMessageSent int64
	lastMessageHash     string
}

func New() (*Controller, error) {
	controller := new(Controller)

	c, err := config.New()
	if err != nil {
		return nil, err
	}

	controller.handler, err = handlers.GetHandler(c)
	if err != nil {
		return nil, err
	}

	if _, err := rest.InClusterConfig(); err != nil {
		controller.kubeClient = utils.GetClientOutOfCluster()
	} else {
		controller.kubeClient = utils.GetClient()
	}

	controller.queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	controller.targets = make(map[string]*Target)

	return controller, nil
}

func (c *Controller) Run() {
	stopCh := make(chan struct{})
	defer close(stopCh)

	for _, v := range []string{"deployment", "statefulset"} {
		informer := newInformer(c.kubeClient, v, c.queue)
		go informer.Run(stopCh)
	}

	go wait.Until(c.processEvents, time.Second, stopCh)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func newInformer(kubeClient kubernetes.Interface, resourceType string, queue workqueue.RateLimitingInterface) cache.SharedIndexInformer {
	var informer cache.SharedIndexInformer
	switch resourceType {
	case "deployment":
		informer = cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.AppsV1().Deployments("").List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.AppsV1().Deployments("").Watch(options)
				},
			},
			&apps_v1.Deployment{},
			0,
			cache.Indexers{},
		)
	case "statefulset":
		informer = cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.AppsV1().StatefulSets("").List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.AppsV1().StatefulSets("").Watch(options)
				},
			},
			&apps_v1.StatefulSet{},
			0,
			cache.Indexers{},
		)
	case "daemonset":
		informer = cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
					return kubeClient.AppsV1().DaemonSets("").List(options)
				},
				WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
					return kubeClient.AppsV1().DaemonSets("").Watch(options)
				},
			},
			&apps_v1.DaemonSet{},
			0,
			cache.Indexers{},
		)
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			var event Event
			event.obj = obj
			event.eventType = "add"
			queue.Add(event)
		},
		UpdateFunc: func(old, new interface{}) {
			var event Event
			event.obj = new
			event.eventType = "update"
			queue.Add(event)
		},
		DeleteFunc: func(obj interface{}) {
			var event Event
			event.obj = obj
			event.eventType = "delete"
			queue.Add(event)
		},
	})
	return informer
}

func (c *Controller) processEvents() {
	for {
		e, quit := c.queue.Get()
		if quit {
			return
		}

		event := e.(Event)

		key, err := cache.MetaNamespaceKeyFunc(event.obj)
		if err != nil {
			logrus.Errorf("Failed to get name: %v", err)
		}

		if deployment, ok := event.obj.(*apps_v1.Deployment); ok {
			c.processResource(key, deployment, deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
		} else if statefulset, ok := event.obj.(*apps_v1.StatefulSet); ok {
			c.processResource(key, statefulset, statefulset.Status.ReadyReplicas, *statefulset.Spec.Replicas)
		} else {
			logrus.Errorf("Unsupported obj: %v", event.obj)
		}
	}
}

func (c *Controller) processResource(key string, resource interface{}, readyReplicas, replicas int32) {
	target, ok := c.targets[key]
	if ok {
		target.obj = resource
		c.generateMessageAndSend(target, true)
	} else {
		if readyReplicas == replicas {
			return
		}
		c.targets[key] = &Target{
			obj: resource,
		}
		c.generateMessageAndSend(c.targets[key], false)
	}
}

func (c *Controller) generateMessageAndSend(target *Target, checkDuplicate bool) {
	if deployment, ok := target.obj.(*apps_v1.Deployment); ok {
		c.doGenerateMessageAndSend(
			target,
			checkDuplicate,
			deployment.GetNamespace(),
			deployment.Status.UpdatedReplicas,
			deployment.Status.ReadyReplicas,
			*deployment.Spec.Replicas,
			deployment.Spec.Selector.MatchLabels,
		)
	} else if statefulset, ok := target.obj.(*apps_v1.StatefulSet); ok {
		c.doGenerateMessageAndSend(
			target,
			checkDuplicate,
			statefulset.GetNamespace(),
			statefulset.Status.UpdatedReplicas,
			statefulset.Status.ReadyReplicas,
			*statefulset.Spec.Replicas,
			statefulset.Spec.Selector.MatchLabels,
		)
	} else {
		logrus.Errorf("Unsupported target obj: %v", target.obj)
	}
}

func (c *Controller) doGenerateMessageAndSend(target *Target, checkDuplicate bool, namespace string, updatedReplicas, readyReplicas, replicas int32, matchLabels map[string]string) {
	key, err := cache.MetaNamespaceKeyFunc(target.obj)
	if err != nil {
		logrus.Errorf("Failed to get name: %v", err)
	}

	var strBuilder strings.Builder
	var str string
	if readyReplicas == replicas {
		str = fmt.Sprintf("is in a healthy state now (%v/%v)", readyReplicas, replicas)
	} else {
		str = fmt.Sprintf("is rolling out an update, %v replicas updated out of %v, %v ready", updatedReplicas, replicas, readyReplicas)
	}
	strBuilder.WriteString(fmt.Sprintf("*%v %v*", key, str))

	pods, err := c.kubeClient.CoreV1().Pods(namespace).List(meta_v1.ListOptions{LabelSelector: labels.Set(matchLabels).String()})
	if err != nil {
		logrus.Errorf("Failed to list pods: %v", err)
	} else {
		for _, v := range pods.Items {
			strBuilder.WriteString(fmt.Sprintf("\n\n\tPod: *%v* Phase: *%v*", v.GetName(), v.Status.Phase))

			for _, container := range v.Status.ContainerStatuses {
				strBuilder.WriteString("\n\t\tContainer: *" + container.Name + "*")
				state := container.State
				var stateStr string
				if state.Waiting != nil {
					stateStr = "Waiting"
				} else if state.Running != nil {
					stateStr = "Running"
				} else if state.Terminated != nil {
					stateStr = "Terminated"
				}
				if len(stateStr) > 0 {
					strBuilder.WriteString(" Status: *" + stateStr + "*")
				}
				strBuilder.WriteString("\n\t\t\tImage Id: " + container.ImageID)
			}
		}
	}

	message := strBuilder.String()
	if checkDuplicate && message == target.lastMessageHash && time.Now().Unix()-target.lastTimeMessageSent < 5 {
		return
	}
	c.handler.Notify(message)
	target.lastTimeMessageSent = time.Now().Unix()
	h := sha256.New()
	h.Write([]byte(message))
	target.lastMessageHash = fmt.Sprintf("%x", h.Sum(nil))
}
