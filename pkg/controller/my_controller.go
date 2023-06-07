package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	baseDelay = 1 * time.Second
	maxDelay  = 5 * time.Second
)

var (
	infoFormat  = fmt.Sprintf("%v INFO message[%%v]", time.Now().String())
	errorFormat = fmt.Sprintf("%v ERROR message[%%v]", time.Now().String())
)

func NewMyController(podInformer coreinformers.PodInformer, kubeclient clientset.Interface) *MyController {
	mc := &MyController{
		kubeClient: kubeclient,
		queue:      workqueue.NewRateLimitingQueueWithConfig(workqueue.NewItemExponentialFailureRateLimiter(baseDelay, maxDelay), workqueue.RateLimitingQueueConfig{Name: "my-controller"}),
	}
	mc.podStoreSynced = podInformer.Informer().HasSynced
	mc.podLister = podInformer.Lister()

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    mc.addPod,
		UpdateFunc: mc.updatePod,
		DeleteFunc: mc.deletePod,
	})
	return mc
}

type MyController struct {
	kubeClient     clientset.Interface
	podStoreSynced cache.InformerSynced
	podLister      corelisters.PodLister
	queue          workqueue.RateLimitingInterface
}

func (mc *MyController) Run(ctx context.Context) {
	defer mc.queue.ShutDown()
	fmt.Printf(infoFormat, "Starting my pod controller")
	defer fmt.Printf(infoFormat, "Shutting down my pod controller")

	if !cache.WaitForNamedCacheSync("pod", ctx.Done(), mc.podStoreSynced) {
		fmt.Printf(errorFormat, "sync for pod failed")
		return
	}

	go wait.UntilWithContext(ctx, mc.worker, time.Second)

	<-ctx.Done()

}

func (mc *MyController) syncPod() error {
	return nil
}

func (mc *MyController) addPod(obj interface{}) {

}

func (mc *MyController) updatePod(old interface{}, cur interface{}) {

}

func (mc *MyController) deletePod(obj interface{}) {

}

func (mc *MyController) worker(ctx context.Context) {
	for mc.processNextWorkItem(ctx) {
	}
}

func (mc *MyController) processNextWorkItem(ctx context.Context) bool {
	key, quit := mc.queue.Get()
	if quit {
		return false
	}
	defer mc.queue.Done(key)

	if err := mc.syncPod(); err != nil {
		fmt.Printf(errorFormat, "process pod [%v] failed", key)
		return false
	}

	fmt.Printf(infoFormat, "process pod [%v] success", key)
	return true
}
