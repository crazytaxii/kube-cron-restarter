module github.com/crazytaxii/kube-cron-restarter

go 1.16

require (
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/robfig/cron v1.2.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.10.1
	k8s.io/api v0.20.15
	k8s.io/apimachinery v0.20.15
	k8s.io/client-go v0.20.15
	k8s.io/klog/v2 v2.60.1
	sigs.k8s.io/controller-runtime v0.6.2
)
