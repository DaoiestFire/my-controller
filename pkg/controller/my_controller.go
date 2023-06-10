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
	"k8s.io/klog/v2"
)

const (
	baseDelay = 1 * time.Second
	maxDelay  = 5 * time.Second
)

func NewMyController(podInformer coreinformers.PodInformer, kubeclient clientset.Interface) *MyController {
	mc := &MyController{
		kubeClient: kubeclient,
		queue:      workqueue.NewRateLimitingQueueWithConfig(workqueue.NewItemExponentialFailureRateLimiter(baseDelay, maxDelay), workqueue.RateLimitingQueueConfig{Name: "my-controller"}),
	}
	mc.podStoreSynced = podInformer.Informer().HasSynced
	mc.podLister = podInformer.Lister()

	// 监听资源的事件，添加对应的处理函数
	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    mc.addPod,
		UpdateFunc: mc.updatePod,
		DeleteFunc: mc.deletePod,
	})

	// 创建两个标签选择器，用来判断pod是否携带对应标签
	selector := &metav1.LabelSelector{
		MatchLabels: map[string]string{"mycontroller": "mycontroller"},
	}
	targetSelector, _ := metav1.LabelSelectorAsSelector(selector)

	mc.targetSelector = targetSelector

	selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{"processed": "processed"},
	}
	resultSelector, _ := metav1.LabelSelectorAsSelector(selector)

	mc.resultSelector = resultSelector
	return mc
}

type MyController struct {
	kubeClient     clientset.Interface
	podStoreSynced cache.InformerSynced
	podLister      corelisters.PodLister
	queue          workqueue.RateLimitingInterface

	targetSelector labels.Selector
	resultSelector labels.Selector
}

func (mc *MyController) Run(ctx context.Context) {
	defer mc.queue.ShutDown()
	klog.Info("Starting my pod controller")
	defer klog.Info("Shutting down my pod controller")

	// 等待缓存同步完成
	if !cache.WaitForNamedCacheSync("pod", ctx.Done(), mc.podStoreSynced) {
		klog.Info("sync for pod failed")
		return
	}

	go wait.UntilWithContext(ctx, mc.worker, time.Second)

	<-ctx.Done()

}

// queue中对象的实际处理逻辑
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
	if mc.isTargetPod(&pod) && !mc.isResultPod(&pod) {
		// 如果pod有标签”mycontroller:mycontroller“但是没有”processed:processed“就将”processed:processed“加上
		podLabels["processed"] = "processed"
		klog.Infof("pod [%v] add processed:processed label", key)
	} else if !mc.isTargetPod(&pod) && mc.isResultPod(&pod) {
		// 如果pod没有标签”mycontroller:mycontroller“但是有”processed:processed“就将”processed:processed“删除
		delete(podLabels, "processed")
		klog.Infof("pod [%v] delete processed:processed label", key)
	} else {
		return nil
	}
	pod.SetLabels(podLabels)

	_, err = mc.kubeClient.CoreV1().Pods(ns).Update(context.TODO(), &pod, metav1.UpdateOptions{})
	if err != nil {
		mc.queue.Add(key)
		return fmt.Errorf("update pod [%v] failed ---> [%v]", key, err)
	}
	klog.Infof("update pod [%v] success", key)

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
		klog.Errorf("get key for obj [%v] failed", key)
		return
	}

	// 这里去匹配pod的labels
	if !mc.isTargetPod(pod) {
		klog.Infof("[%v] is not target pod", key)
		return
	}

	mc.queue.Add(key)
	klog.Infof("add pod [%v] to queue success", key)
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
		klog.Errorf("get key for obj [%v] failed", key)
		return
	}

	isOldMatch := mc.isTargetPod(oldPod)
	isCurMatch := mc.isTargetPod(curPod)

	if (isOldMatch && isCurMatch) || (!isOldMatch && !isCurMatch) {
		klog.Infof("pod [%v] no need process", key)
		return
	}

	mc.queue.Add(key)
	klog.Infof("add pod [%v] to queue success,isOldMatch[%v] isCurMatch [%v]", key, isOldMatch, isCurMatch)
}

// 删除pod打印日志即可
func (mc *MyController) deletePod(obj interface{}) {
	pod := obj.(*v1.Pod)
	// 获取pod的key。即”namespace：pod name“
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(pod)
	if err != nil {
		klog.Errorf("get key for obj [%v] failed", key)
		return
	}

	klog.Infof("pod [%v] is deleted", key)
}

func (mc *MyController) worker(ctx context.Context) {
	for mc.processNextWorkItem(ctx) {
	}
}

// 从队列中取一个item进程处理，并标记为完成
func (mc *MyController) processNextWorkItem(ctx context.Context) bool {
	key, quit := mc.queue.Get()
	if quit {
		return false
	}
	defer mc.queue.Done(key)

	if err := mc.syncPod(key.(string)); err != nil {
		klog.Errorf("process pod [%v] failed ---> [%v]", key, err)
		return false
	}

	klog.Infof("process pod [%v] success", key)
	return true
}

// 判断一个pod是否包含标签 ”mycontroller:mycontroller“
func (mc *MyController) isTargetPod(pod *v1.Pod) bool {
	return mc.targetSelector.Matches(labels.Set(pod.GetLabels()))
}

// 判断一个pod是否包含标签 ”processed:processed“
func (mc *MyController) isResultPod(pod *v1.Pod) bool {
	return mc.resultSelector.Matches(labels.Set(pod.GetLabels()))
}
