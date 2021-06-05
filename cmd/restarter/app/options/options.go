package options

import (
	"time"

	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app/config"
	"github.com/crazytaxii/kube-cron-restarter/pkg/restarter"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	componentBaseConfig "k8s.io/component-base/config"
	"k8s.io/klog/v2"
)

type AutoRestarterOptions struct {
	HealthzPort int
	// config of leader election client
	LeaderElection componentBaseConfig.LeaderElectionConfiguration
	ResyncPeriod   time.Duration
	// CronJobNamespace string // all relative CronJobs will put into this namespace
	// KubeCtlImage     string // image with kubectl
	restarter.ControllerOptions
}

const (
	DefaultHealthzPort       = 8080
	DefaultLeaderElect       = false
	DefaultLeaseDuration     = 30
	DefaultRenewDeadline     = 15
	DefaultRetryPeriod       = 5
	DefaultResyncPeriod      = 1 * time.Hour
	DefaultResourceLock      = "endpointsleases"
	DefaultResourceName      = "cron-restarter-controller"
	DefaultResourceNamespace = "kube-system"

	AutoRestarterControllerUserAgent = "cron-restarter-controller"
)

var (
	leaseDuration int
	renewDeadline int
	retryPeriod   int
)

func NewAutoRestarterOptions() *AutoRestarterOptions {
	return &AutoRestarterOptions{
		HealthzPort:       DefaultHealthzPort,
		LeaderElection:    componentBaseConfig.LeaderElectionConfiguration{},
		ResyncPeriod:      DefaultResyncPeriod,
		ControllerOptions: restarter.ControllerOptions{},
	}
}

func (o *AutoRestarterOptions) BindFlags(cmd *cobra.Command) {
	// healthz config
	cmd.Flags().IntVarP(&o.HealthzPort, "healthz-port", "", DefaultHealthzPort, "The port of healthz server")

	// leader election
	cmd.Flags().StringVarP(&o.LeaderElection.ResourceLock, "resource-lock", "", DefaultResourceLock, "")
	cmd.Flags().StringVarP(&o.LeaderElection.ResourceName, "resource-name", "", DefaultResourceName, "")
	cmd.Flags().StringVarP(&o.LeaderElection.ResourceNamespace, "resource-namespace", "", DefaultResourceNamespace, "")
	cmd.Flags().BoolVarP(&o.LeaderElection.LeaderElect, "leader-elect", "", DefaultLeaderElect, "")
	cmd.Flags().IntVarP(&leaseDuration, "lease-duration", "", DefaultLeaseDuration, "")
	cmd.Flags().IntVarP(&renewDeadline, "renew-deadline", "", DefaultRenewDeadline, "")
	cmd.Flags().IntVarP(&retryPeriod, "retry-period", "", DefaultRetryPeriod, "")
	cmd.Flags().DurationVarP(&o.ResyncPeriod, "resync-period", "", DefaultResyncPeriod, "")

	// controller options
	// namespace CronJobs put in
	cmd.Flags().StringVarP(&o.CronJobNamespace, "cronjob-namespace", "", restarter.DefaultCronJobNamespace, "")
	// kubectl image
	cmd.Flags().StringVarP(&o.KubeCtlImage, "kubectl-image", "", restarter.DefaultKubeCtlImage, "")
}

func (o *AutoRestarterOptions) buildLeaderClient(clientConfig *restclient.Config, name string) (clientset.Interface, error) {
	restConfig := restclient.AddUserAgent(clientConfig, name)
	return clientset.NewForConfig(restConfig)
}

func (o *AutoRestarterOptions) Config() (*config.AutoRestarterConfig, error) {
	kubeConfig, err := config.BuildKubeConfig()
	if err != nil {
		return nil, err
	}

	lc, err := o.buildLeaderClient(kubeConfig, "leader-client")
	if err != nil {
		return nil, err
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: lc.CoreV1().Events("")})
	er := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: AutoRestarterControllerUserAgent})

	return &config.AutoRestarterConfig{
		HealthzPort:   o.HealthzPort,
		EventRecorder: er,
		LeaderClient:  lc,
		LeaderElection: componentBaseConfig.LeaderElectionConfiguration{
			LeaderElect:       o.LeaderElection.LeaderElect,
			LeaseDuration:     metav1.Duration{time.Duration(leaseDuration) * time.Second},
			RenewDeadline:     metav1.Duration{time.Duration(renewDeadline) * time.Second},
			RetryPeriod:       metav1.Duration{time.Duration(retryPeriod) * time.Second},
			ResourceLock:      o.LeaderElection.ResourceLock,
			ResourceName:      o.LeaderElection.ResourceName,
			ResourceNamespace: o.LeaderElection.ResourceNamespace,
		},
		ControllerOptions: o.ControllerOptions,
	}, nil
}
