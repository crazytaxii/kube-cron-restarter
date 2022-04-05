package controller

import (
	"errors"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type (
	AutoRestarterContext struct {
		Name           string
		Namespace      string
		Kind           string
		Schedule       string // optional
		Image          string // optional
		Object         metav1.Object
		ServiceAccount string            // optional
		Labels         map[string]string // optional
	}
	AutoRestarterContextOptions func(*AutoRestarterContext)
)

func NewAutoRestarterContext(obj interface{}, opts ...AutoRestarterContextOptions) *AutoRestarterContext {
	arCtx := &AutoRestarterContext{}
	switch v := obj.(type) {
	case *appsv1.Deployment:
		arCtx.Name = v.Name
		arCtx.Namespace = v.Namespace
		arCtx.Kind = "Deployment"
		arCtx.Object = v
	case *appsv1.StatefulSet:
		arCtx.Name = v.Name
		arCtx.Namespace = v.Namespace
		arCtx.Kind = "StatefulSet"
		arCtx.Object = v
	case *batchv1beta1.CronJob:
		arCtx.Name = v.Name
		arCtx.Namespace = v.Namespace
		arCtx.Kind = "CronJob"
		arCtx.Object = v
	default:
		return nil
	}

	for _, o := range opts {
		o(arCtx)
	}
	return arCtx
}

func WithSchedule(schedule string) AutoRestarterContextOptions {
	return func(o *AutoRestarterContext) {
		o.Schedule = schedule
	}
}

func WithImage(image string) AutoRestarterContextOptions {
	return func(o *AutoRestarterContext) {
		o.Image = image
	}
}

func WithServiceAccount(serviceAccount string) AutoRestarterContextOptions {
	return func(o *AutoRestarterContext) {
		o.ServiceAccount = serviceAccount
	}
}

func WithLabels(labels map[string]string) AutoRestarterContextOptions {
	return func(o *AutoRestarterContext) {
		o.Labels = labels
	}
}

func (ctx *AutoRestarterContext) Log(operation string) {
	klog.Infof("%s %s %s/%s", operation, ctx.Kind, ctx.Namespace, ctx.Name)
}

// Join namespace, name and kind for the name of our CronJob
func joinCronJobName(namespace, name, kind string) string {
	return fmt.Sprintf("%s-%s-%s-restarter", namespace, name, strings.ToLower(kind))
}

func getScheduleFromAnnotations(annotations map[string]string, want string) string {
	schedule, ok := annotations[want]
	if !ok {
		return ""
	}
	return schedule
}

// Split namespace, name and kind from the name of our CronJob
func splitCronJobName(cronJobName string) (namespace, name, kind string) {
	parts := strings.Split(cronJobName, "-")
	if len(parts) >= 4 {
		return parts[0], parts[1], parts[2]
	}
	return
}

// metaKeyFunc joins namespace/name/kind to a key
func metaKeyFunc(obj interface{}) (string, error) {
	if arCtx := NewAutoRestarterContext(obj); arCtx != nil {
		return fmt.Sprintf("%s/%s/%s", arCtx.Namespace, arCtx.Name, arCtx.Kind), nil
	}
	return "", errors.New("KeyFunc only support Deployment/StatefulSet")
}

func deletionHandlingKeyFunc(obj interface{}) (string, error) {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	return metaKeyFunc(obj)
}

// splitMetaKey splits key to namespace, name and kind
func splitMetaKey(key string) (namespace, name, kind string, err error) {
	parts := strings.Split(key, "/")
	switch len(parts) {
	case 1:
		// name only, no namespace
		return "", parts[0], "", nil
	case 2:
		// namespace and name
		return parts[0], parts[1], "", nil
	case 3:
		// namespace, name and kind
		return parts[0], parts[1], parts[2], nil
	}

	return "", "", "", fmt.Errorf("unexpected key format: %q", key)
}
