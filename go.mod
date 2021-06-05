module github.com/crazytaxii/kube-cron-restarter

go 1.16

require (
	github.com/robfig/cron v1.2.0
	github.com/spf13/cobra v1.1.3
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.19.2
	k8s.io/component-base v0.19.2
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/controller-runtime v0.6.2
)
