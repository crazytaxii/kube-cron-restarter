package restarter

import (
	"context"
	goerrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/crazytaxii/kube-cron-restarter/pkg/utils/sliceutil"
	"github.com/robfig/cron"
	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	batchListers "k8s.io/client-go/listers/batch/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	ControllerName             = "cron-restarter-controller"
	AutoRestarterAnnotationKey = "cronRestart"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"
	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists      = "Resource %q already exists and is not managed by Deployment"
	AutoRestarterFinalizer     = "cron-restarter.io/finalizer"
	AutoRestarterContainerName = "restarter"
	ServiceAccountName         = "cron-restarter"

	DefaultCronJobNamespace = "kube-system"
	DefaultKubeCtlImage     = "bitnami/kubectl:1.18.6"
)

type (
	RestarterController struct {
		// clientset clientset.Interface
		kubeclientset kubernetes.Interface
		// recorder is an event recorder for recording Event resources to the Kubernetes API.
		eventRecorder record.EventRecorder

		deploymentsLister  appslisters.DeploymentLister
		deploymentsSynced  cache.InformerSynced
		statefulSetsLister appslisters.StatefulSetLister
		statefulSetsSynced cache.InformerSynced
		cronJobsLister     batchListers.CronJobLister
		cronJobsSynced     cache.InformerSynced

		workqueue           workqueue.RateLimitingInterface
		kubeInformerFactory kubeinformers.SharedInformerFactory
		ControllerOptions
	}
	ControllerOptions struct {
		CronJobNamespace string // all relative CronJobs will put into this namespace
		KubeCtlImage     string // image with kubectl
	}
	ControllerOptionsFunc func(*ControllerOptions)
)

func NewDefaultControllerOptions() *ControllerOptions {
	return &ControllerOptions{
		CronJobNamespace: DefaultCronJobNamespace,
		KubeCtlImage:     DefaultKubeCtlImage,
	}
}

func WithCronJobNamespace(namespace string) ControllerOptionsFunc {
	return func(o *ControllerOptions) {
		o.CronJobNamespace = namespace
	}
}

func WithKubeCtlImage(image string) ControllerOptionsFunc {
	return func(o *ControllerOptions) {
		o.KubeCtlImage = image
	}
}

func NewRestarterController(
	kubeClientset kubernetes.Interface,
	resyncPeriod time.Duration,
	controllerOptions ...ControllerOptionsFunc,
) (*RestarterController, error) {
	// Create event broadcaster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClientset, resyncPeriod)
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	statefulsetInformer := kubeInformerFactory.Apps().V1().StatefulSets()
	cronJobInformer := kubeInformerFactory.Batch().V1beta1().CronJobs()

	opts := NewDefaultControllerOptions()
	for _, o := range controllerOptions {
		o(opts)
	}

	rc := &RestarterController{
		kubeclientset:       kubeClientset,
		eventRecorder:       eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: ControllerName}),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cron-restarter"),
		kubeInformerFactory: kubeInformerFactory,
		ControllerOptions:   *opts,
	}

	// Set up event handlers for Deployments change
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rc.addDeploymentFunc,
		UpdateFunc: rc.updateDeploymentFunc,
		DeleteFunc: rc.deleteDeploymentFunc,
	})

	// Set up event handlers for StatefulSets change
	statefulsetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rc.addStatefulSetFunc,
		UpdateFunc: rc.updateStatefulSetFunc,
		DeleteFunc: rc.deleteStatefulSetFunc,
	})

	// Set up event handlers for CronJobs change
	cronJobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    rc.addCronJobFunc,
		UpdateFunc: rc.updateCronJobFunc,
		DeleteFunc: rc.deleteCronJobFunc,
	})

	rc.deploymentsLister = deploymentInformer.Lister()
	rc.statefulSetsLister = statefulsetInformer.Lister()
	rc.cronJobsLister = cronJobInformer.Lister()

	rc.deploymentsSynced = deploymentInformer.Informer().HasSynced
	rc.statefulSetsSynced = statefulsetInformer.Informer().HasSynced
	rc.cronJobsSynced = cronJobInformer.Informer().HasSynced

	return rc, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (rc *RestarterController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer rc.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Auto Restarter controller")

	rc.kubeInformerFactory.Start(stopCh)

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, rc.deploymentsSynced); !ok {
		return goerrors.New("Failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(rc.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (rc *RestarterController) runWorker() {
	for rc.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (rc *RestarterController) processNextWorkItem() bool {
	obj, shutdown := rc.workqueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer rc.workqueue.Done(obj)
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		key, ok := obj.(string)
		if !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			rc.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("Expected string in workqueue but got %#v", obj))
			return nil
		}

		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := rc.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			rc.workqueue.AddRateLimited(key)
			return fmt.Errorf("Syncing err '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		rc.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Foo resource
// with the current status of the resource.
func (rc *RestarterController) syncHandler(key string) error {
	// Convert the namespace/name/kind string into a distinct namespace, name and kind
	namespace, name, kind, err := splitMetaKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	namespaceMeta := namespace + "/" + name

	// Get the Deployment/StatefulSet with this namespace/name/kind
	obj, err := rc.fetchObject(namespace, name, kind)
	if err != nil {
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("%s '%s' in work queue no longer exists", kind, namespaceMeta))
			return nil
		}
		return err
	}

	// Get value of AutoRestarterAnnotationKey in annotations map
	schedule := getScheduleFromAnnotations(obj.GetAnnotations(), AutoRestarterAnnotationKey)
	if schedule == "" {
		klog.Infof("%s '%s' is no need to restart", kind, namespaceMeta)
		return nil
	}
	// Validate cron expression
	if _, err := cron.ParseStandard(schedule); err != nil {
		utilruntime.HandleError(fmt.Errorf("Invalid cron expression '%s'", schedule))
		return nil
	}

	arCtx := NewAutoRestarterContext(obj, WithSchedule(schedule), WithImage(rc.KubeCtlImage),
		WithServiceAccount(ServiceAccountName), WithLabels(obj.GetLabels()))
	if obj.GetDeletionTimestamp().IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object.
		if !sliceutil.ContainString(obj.GetFinalizers(), AutoRestarterFinalizer) {
			// add a finalizer to it
			obj.SetFinalizers(append(obj.GetFinalizers(), AutoRestarterFinalizer))
			if err := rc.updateObject(obj); err != nil {
				utilruntime.HandleError(fmt.Errorf("Updating %s '%s' err %s", kind, namespaceMeta, err.Error()))
				return err
			}
		}
	} else {
		// The object is being deleted
		if sliceutil.ContainString(obj.GetFinalizers(), AutoRestarterFinalizer) {
			if err := rc.syncCronJobForDeletion(*arCtx); err != nil {
				return err
			}

			// Remove our finalizer from the list and update it.
			obj.SetFinalizers(sliceutil.RemoveString(obj.GetFinalizers(), AutoRestarterFinalizer))
			if err := rc.updateObject(obj); err != nil {
				utilruntime.HandleError(fmt.Errorf("Updating %s '%s' err %s", kind, namespaceMeta, err.Error()))
				return err
			}
		}
		return nil
	}

	cronJob, err := rc.syncCronJob(*arCtx)
	if err != nil {
		return err
	}

	// If the CronJob is not controlled by this Deployment resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(cronJob, obj) {
		errMsg := fmt.Sprintf(MessageResourceExists, obj.GetName())
		switch v := obj.(type) {
		case *appsv1.Deployment:
			rc.eventRecorder.Event(v, corev1.EventTypeWarning, ErrResourceExists, errMsg)
		case *appsv1.StatefulSet:
			rc.eventRecorder.Event(v, corev1.EventTypeWarning, ErrResourceExists, errMsg)
		}
		return goerrors.New(errMsg)
	}

	return err
}

// Get the Deployment/StatefulSet with this namespace/name/kind
func (rc *RestarterController) fetchObject(namespace, name, kind string) (metav1.Object, error) {
	switch strings.ToLower(kind) {
	case "deployment":
		return rc.deploymentsLister.Deployments(namespace).Get(name)
	case "statefulset":
		return rc.statefulSetsLister.StatefulSets(namespace).Get(name)
	default:
		return nil, nil // never reached
	}
}

func (rc *RestarterController) syncCronJob(arCtx AutoRestarterContext) (*batchv1beta1.CronJob, error) {
	// Get the CronJob with namespace, name and kind of it's owner
	name := joinCronJobName(arCtx.Namespace, arCtx.Name, arCtx.Kind)
	cronJob, err := rc.cronJobsLister.CronJobs(rc.CronJobNamespace).Get(name)
	if errors.IsNotFound(err) {
		// Create the CronJob when it is not found
		cronJob, err = rc.kubeclientset.BatchV1beta1().CronJobs(rc.CronJobNamespace).Create(
			context.TODO(), newCronJob(arCtx), metav1.CreateOptions{})
	}
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}
	return cronJob, nil
}

func (rc *RestarterController) syncCronJobForDeletion(arCtx AutoRestarterContext) error {
	name := joinCronJobName(arCtx.Namespace, arCtx.Name, arCtx.Kind)
	return rc.kubeclientset.BatchV1beta1().CronJobs(rc.CronJobNamespace).Delete(context.TODO(),
		name, metav1.DeleteOptions{})
}

// enqueue takes a Deployment/StatefulSet resource and converts it into a namespace/name
// string which is then put onto the work queue.
func (rc *RestarterController) enqueue(obj interface{}) {
	key, err := metaKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	rc.workqueue.Add(key)
}

func (rc *RestarterController) enqueueForDeletion(obj interface{}) {
	key, err := deletionHandlingKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	rc.workqueue.Add(key)
}

// handleCronJob will take any resource implementing metav1.Object and attempt
// to find the Deployment/StatefulSet resource that 'owns' it.
func (rc *RestarterController) handleCronJob(obj interface{}) {
	object, ok := obj.(metav1.Object)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Deployment/StatefulSet, we should not do anything more
		// with it.
		if strings.ToLower(ownerRef.Kind) != "deployment" &&
			strings.ToLower(ownerRef.Kind) != "statefulset" {
			return
		}

		ownerNamespace, _, _ := splitCronJobName(object.GetName())
		switch strings.ToLower(ownerRef.Kind) {
		case "deployment":
			owner, err := rc.deploymentsLister.Deployments(ownerNamespace).Get(ownerRef.Name)
			if err != nil {
				klog.V(4).Infof("ignoring orphaned object '%s' of deployment '%s'", object.GetSelfLink(), ownerRef.Name)
				return
			}
			rc.enqueue(owner)
		case "statefulset":
			owner, err := rc.statefulSetsLister.StatefulSets(ownerNamespace).Get(ownerRef.Name)
			if err != nil {
				klog.V(4).Infof("ignoring orphaned object '%s' of statefulset '%s'", object.GetSelfLink(), ownerRef.Name)
				return
			}
			rc.enqueue(owner)
		}

		return
	}
}

func (rc *RestarterController) updateObject(obj interface{}) error {
	var err error
	ctx := context.TODO()
	switch o := obj.(type) {
	case *appsv1.Deployment:
		_, err = rc.kubeclientset.AppsV1().Deployments(o.GetNamespace()).Update(ctx, o, metav1.UpdateOptions{})
	case *appsv1.StatefulSet:
		_, err = rc.kubeclientset.AppsV1().StatefulSets(o.GetNamespace()).Update(ctx, o, metav1.UpdateOptions{})
	default:
		return nil
	}
	return err
}

func (rc *RestarterController) addDeploymentFunc(obj interface{}) {
	if arCtx := NewAutoRestarterContext(obj); arCtx != nil {
		arCtx.Log("Adding")
	}
	rc.enqueue(obj)
}

func (rc *RestarterController) updateDeploymentFunc(old, new interface{}) {
	if old.(*appsv1.Deployment).ResourceVersion == new.(*appsv1.Deployment).ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}
	if arCtx := NewAutoRestarterContext(new); arCtx != nil {
		arCtx.Log("Updating")
	}
	rc.enqueue(new)
}

func (rc *RestarterController) deleteDeploymentFunc(obj interface{}) {
	if arCtx := NewAutoRestarterContext(obj); arCtx != nil {
		arCtx.Log("Deleting")
	}
	rc.enqueueForDeletion(obj)
}

func (rc *RestarterController) addStatefulSetFunc(obj interface{}) {
	if arCtx := NewAutoRestarterContext(obj); arCtx != nil {
		arCtx.Log("Adding")
	}
	rc.enqueue(obj)
}

func (rc *RestarterController) updateStatefulSetFunc(old, new interface{}) {
	if old.(*appsv1.StatefulSet).ResourceVersion == new.(*appsv1.StatefulSet).ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}
	if arCtx := NewAutoRestarterContext(new); arCtx != nil {
		arCtx.Log("Updating")
	}
	rc.enqueue(new)
}

func (rc *RestarterController) deleteStatefulSetFunc(obj interface{}) {
	if arCtx := NewAutoRestarterContext(obj); arCtx != nil {
		arCtx.Log("Deleting")
	}
	rc.enqueueForDeletion(obj)
}

func (rc *RestarterController) addCronJobFunc(obj interface{}) {
	if arCtx := NewAutoRestarterContext(obj); arCtx != nil {
		arCtx.Log("Adding")
	}
	rc.handleCronJob(obj)
}

func (rc *RestarterController) updateCronJobFunc(old, new interface{}) {
	if old.(*batchv1beta1.CronJob).ResourceVersion == new.(*batchv1beta1.CronJob).ResourceVersion {
		// Periodic resync will send update events for all known Deployments.
		// Two different versions of the same Deployment will always have different RVs.
		return
	}
	if arCtx := NewAutoRestarterContext(new); arCtx != nil {
		arCtx.Log("Updating")
	}
	rc.handleCronJob(new)
}

func (rc *RestarterController) deleteCronJobFunc(obj interface{}) {
	if arCtx := NewAutoRestarterContext(obj); arCtx != nil {
		arCtx.Log("Deleting")
	}
	rc.handleCronJob(obj)
}
