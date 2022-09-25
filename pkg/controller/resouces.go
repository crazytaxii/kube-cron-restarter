package controller

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultBackoffLimit int32 = 3

func newCronJob(arCtx AutoRestarterContext) *batchv1beta1.CronJob {
	return &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:   joinCronJobName(arCtx.Namespace, arCtx.Name, arCtx.Kind),
			Labels: arCtx.Labels,
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule:    arCtx.Schedule,
			JobTemplate: newJobTemplate(arCtx),
		},
	}
}

func newJobTemplate(arCtx AutoRestarterContext) batchv1beta1.JobTemplateSpec {
	backoffLimit := defaultBackoffLimit
	return batchv1beta1.JobTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: arCtx.Labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: arCtx.Labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						newKubectlContainer(arCtx),
					},
					RestartPolicy:      v1.RestartPolicyOnFailure, // restart on failure
					ServiceAccountName: arCtx.ServiceAccount,      // RBAC
				},
			},
		},
	}
}

func newKubectlContainer(arCtx AutoRestarterContext) v1.Container {
	return v1.Container{
		Name:            AutoRestarterContainerName,
		Image:           arCtx.Image,
		ImagePullPolicy: v1.PullIfNotPresent,
		Command: []string{"/bin/sh", "-c",
			fmt.Sprintf("kubectl rollout restart %s %s -n %s", arCtx.Kind, arCtx.Name, arCtx.Namespace)}, // kubectl rollout restart deployments/statefulsets name
	}
}
