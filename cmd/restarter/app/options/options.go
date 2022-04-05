package options

import (
	"time"

	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app/config"
	"github.com/crazytaxii/kube-cron-restarter/pkg/controller"
	"github.com/spf13/cobra"
)

type AutoRestarterOptions struct {
	// The controller will load config from this file
	ConfigFile  string
	HealthzPort int
	// config of leader election client
	config.LeaderElection
	ResyncPeriod time.Duration
	*controller.ControllerOptions
}

func NewAutoRestarterOptions() *AutoRestarterOptions {
	return &AutoRestarterOptions{
		HealthzPort:       config.DefaultHealthzPort,
		LeaderElection:    config.NewDefaultLeaderElection(),
		ResyncPeriod:      config.DefaultResyncPeriod,
		ControllerOptions: controller.NewDefaultControllerOptions(),
	}
}

func (o *AutoRestarterOptions) BindFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.ConfigFile, "config-file", "", "", "The config file path of this controller")
	// healthz config
	cmd.Flags().IntVarP(&o.HealthzPort, "healthz-port", "", o.HealthzPort, "The port of healthz server")
	// leader election
	cmd.Flags().BoolVar(&o.LeaderElect, "leader-elect", o.LeaderElect, ""+
		"Start a leader election client and gain leadership before "+
		"executing the main loop. Enable this when running replicated "+
		"components for high availability.")
	cmd.Flags().DurationVar(&o.LeaseDuration, "leader-elect-lease-duration", o.LeaseDuration, ""+
		"The duration that non-leader candidates will wait after observing a leadership "+
		"renewal until attempting to acquire leadership of a led but unrenewed leader "+
		"slot. This is effectively the maximum duration that a leader can be stopped "+
		"before it is replaced by another candidate. This is only applicable if leader "+
		"election is enabled.")
	cmd.Flags().DurationVar(&o.RenewDeadline, "leader-elect-renew-deadline", o.RenewDeadline, ""+
		"The interval between attempts by the acting master to renew a leadership slot "+
		"before it stops leading. This must be less than or equal to the lease duration. "+
		"This is only applicable if leader election is enabled.")
	cmd.Flags().DurationVar(&o.RetryPeriod, "leader-elect-retry-period", o.RetryPeriod, ""+
		"The duration the clients should wait between attempting acquisition and renewal "+
		"of a leadership. This is only applicable if leader election is enabled.")
	cmd.Flags().StringVar(&o.ResourceLock, "leader-elect-resource-lock", o.ResourceLock, ""+
		"The type of resource object that is used for locking during "+
		"leader election. Supported options are 'endpoints', 'configmaps', "+
		"'leases', 'endpointsleases' and 'configmapsleases'.")
	cmd.Flags().StringVar(&o.ResourceName, "leader-elect-resource-name", o.ResourceName, ""+
		"The name of resource object that is used for locking during "+
		"leader election.")
	cmd.Flags().StringVar(&o.ResourceNamespace, "leader-elect-resource-namespace", o.ResourceNamespace, ""+
		"The namespace of resource object that is used for locking during "+
		"leader election.")
	cmd.Flags().DurationVarP(&o.ResyncPeriod, "resync-period", "", o.ResyncPeriod, "The duration of resync period")

	// controller options
	controller.BindControllerOptionsFlags(o.ControllerOptions, cmd.Flags())
}

func (o *AutoRestarterOptions) Config() (*config.AutoRestarterConfig, error) {
	if o.ConfigFile != "" {
		return config.LoadConfigFile(o.ConfigFile)
	}

	return &config.AutoRestarterConfig{
		HealthzPort:       o.HealthzPort,
		ResyncPeriod:      o.ResyncPeriod,
		LeaderElection:    o.LeaderElection,
		ControllerOptions: o.ControllerOptions,
	}, nil
}
