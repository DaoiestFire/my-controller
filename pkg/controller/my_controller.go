package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"mycontroller": "mycontroller"},
	}
	resultSelector, _ := metav1.LabelSelectorAsSelector(selector)

	mc.selector = resultSelector
	return mc
}

type MyController struct {
	kubeClient     clientset.Interface
	podStoreSynced cache.InformerSynced
	podLister      corelisters.PodLister
	queue          workqueue.RateLimitingInterface

	selector labels.Selector
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

func (mc *MyController) syncPod(key string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	if len(ns) == 0 || len(name) == 0 {
		return fmt.Errorf("invalid pod key %q: either namespace or name is missing", key)
	}

	sharedPod, err := mc.podLister.Pods(ns).Get(name)
	if err != nil {
		return fmt.Errorf("get pod [%s] from lister failed ---> [%v]", key, err)
	}

	// 深拷贝，避免修改缓存中的对象
	pod := *sharedPod.DeepCopy()
	podLabels := pod.GetLabels()
	// 如果匹配了把processed:processed标签加上
	if mc.isTargetPod(&pod) {
		podLabels["processed"] = "processed"
		fmt.Printf(infoFormat, fmt.Sprintf("pod [%v] add processed:processed label", key))
	} else {
		// 反之，将标签删除
		delete(podLabels, "processed")
		fmt.Printf(infoFormat, fmt.Sprintf("pod [%v] delete processed:processed label", key))
	}
	pod.SetLabels(podLabels)

	_, err = mc.kubeClient.CoreV1().Pods(ns).Update(context.TODO(), &pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update pod [%v] failed", key)
	}

	return nil
}

// 新pod如果携带”mycontroller:mycontroller“标签，就需要处理，置入处理队列中
func (mc *MyController) addPod(obj interface{}) {
	pod := obj.(*v1.Pod)

	//如果已经删除了就不处理
	if pod.DeletionTimestamp != nil {
		mc.deletePod(pod)
		return
	}

	// 获取pod的key。即”namespace：pod name“
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod)
	if err != nil {
		fmt.Printf(errorFormat, fmt.Sprintf("get key for obj [%v] failed", key))
		return
	}

	// 这里去匹配pod的labels
	if !mc.isTargetPod(pod) {
		fmt.Printf(errorFormat, fmt.Sprintf("[%v] is not target pod", key))
		return
	}

	mc.queue.Add(key)
	fmt.Printf(infoFormat, fmt.Sprintf("add pod [%v] to queue success", key))
}

// 如果一个pod原来没有”mycontroller:mycontroller“标签，后续修改其内容将标签加上了，就需要处理，置入处理队列中
// 如果一个pod原来有”mycontroller:mycontroller“标签，现在没有了。我们要删除添加上的标签，也需要置入处理队列
func (mc *MyController) updatePod(old interface{}, cur interface{}) {
	oldPod := old.(*v1.Pod)
	curPod := cur.(*v1.Pod)
	//如果已经删除了就不处理
	if curPod.DeletionTimestamp != nil {
		mc.deletePod(curPod)
		return
	}

	// 获取pod的key。即”namespace：pod name“
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(curPod)
	if err != nil {
		fmt.Printf(errorFormat, fmt.Sprintf("get key for obj [%v] failed", key))
		return
	}

	isOldMatch := mc.isTargetPod(oldPod)
	isCurMatch := mc.isTargetPod(curPod)

	if (isOldMatch && isCurMatch) || (!isOldMatch && !isCurMatch) {
		fmt.Printf(infoFormat, fmt.Sprintf("pod [%v] no need process", key))
	}

	mc.queue.Add(key)
	fmt.Printf(infoFormat, fmt.Sprintf("add pod [%v] to queue success,isOldMatch[%v] isCurMatch [%v]", key, isOldMatch, isCurMatch))
}

// 删除pod就打印日志即可
func (mc *MyController) deletePod(obj interface{}) {
	pod := obj.(*v1.Pod)
	// 获取pod的key。即”namespace：pod name“
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod)
	if err != nil {
		fmt.Printf(errorFormat, fmt.Sprintf("get key for obj [%v] failed", key))
		return
	}

	fmt.Printf(infoFormat, fmt.Sprintf("pod [%v] is deleted", key))
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

	if err := mc.syncPod(key.(string)); err != nil {
		fmt.Printf(errorFormat, "process pod [%v] failed ---> [%v]", key, err)
		return false
	}

	fmt.Printf(infoFormat, "process pod [%v] success", key)
	return true
}

func (mc *MyController) isTargetPod(pod *v1.Pod) bool {
	return mc.selector.Matches(labels.Set(pod.GetLabels()))
}
