package config

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/crazytaxii/kube-cron-restarter/pkg/controller"
	"github.com/spf13/viper"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	defaultKubeConfigDir     = ".kube/config"
	defaultConfigDir         = "."
	DefaultHealthzPort       = 8080
	DefaultResyncPeriod      = 10 * time.Minute
	DefaultLeaseDuration     = 30 * time.Second
	DefaultRenewDeadline     = 15 * time.Second
	DefaultRetryPeriod       = 5 * time.Second
	DefaultResourceName      = "cron-restarter-controller"
	DefaultResourceLock      = "endpointsleases"
	DefaultResourceNamespace = "kube-system"
)

type (
	LeaderElection struct {
		// leaderElect enables a leader election client to gain leadership
		// before executing the main loop. Enable this when running replicated
		// components for high availability.
		LeaderElect bool `json:"leader_elect" yaml:"leaderElect"`
		// leaseDuration is the duration that non-leader candidates will wait
		// after observing a leadership renewal until attempting to acquire
		// leadership of a led but unrenewed leader slot. This is effectively the
		// maximum duration that a leader can be stopped before it is replaced
		// by another candidate. This is only applicable if leader election is
		// enabled.
		LeaseDuration time.Duration `json:"lease_duration" yaml:"leaseDuration"`
		// renewDeadline is the interval between attempts by the acting master to
		// renew a leadership slot before it stops leading. This must be less
		// than or equal to the lease duration. This is only applicable if leader
		// election is enabled.
		RenewDeadline time.Duration `json:"renew_deadline" yaml:"renewDeadline"`
		// retryPeriod is the duration the clients should wait between attempting
		// acquisition and renewal of a leadership. This is only applicable if
		// leader election is enabled.
		RetryPeriod time.Duration `json:"retry_period" yaml:"retryPeriod"`
		// resourceLock indicates the resource object type that will be used to lock
		// during leader election cycles.
		ResourceLock string `json:"resource_lock" yaml:"resourceLock"`
		// resourceName indicates the name of resource object that will be used to lock
		// during leader election cycles.
		ResourceName string `json:"resource_name" yaml:"resourceName"`
		// resourceNamespace indicates the namespace of resource object that will be used to lock
		// during leader election cycles.
		ResourceNamespace string `json:"resource_namespace" yaml:"resourceNamespace"`
	}
	AutoRestarterConfig struct {
		HealthzPort    int                                            `json:"healthz_port" yaml:"healthzPort"` // the port health server listening on
		LeaderElection `json:"leader_election" yaml:"leaderElection"` // config of leader election client
		ResyncPeriod   time.Duration                                  `json:"resync_period" yaml:"resyncPeriod"` // resync prriod
		*controller.ControllerOptions
	}
)

func NewDefaultLeaderElection() LeaderElection {
	return LeaderElection{
		LeaderElect:       true,
		LeaseDuration:     DefaultLeaseDuration,
		RenewDeadline:     DefaultRenewDeadline,
		RetryPeriod:       DefaultRetryPeriod,
		ResourceLock:      DefaultResourceLock,
		ResourceName:      DefaultResourceName,
		ResourceNamespace: DefaultResourceNamespace,
	}
}

func NewDefaultConfig() *AutoRestarterConfig {
	return &AutoRestarterConfig{
		HealthzPort:       DefaultHealthzPort,
		ResyncPeriod:      DefaultResyncPeriod,
		LeaderElection:    NewDefaultLeaderElection(),
		ControllerOptions: controller.NewDefaultControllerOptions(),
	}
}

func BuildKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	return clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), defaultKubeConfigDir))
}

func LoadConfigFile(name string) (*AutoRestarterConfig, error) {
	configFile, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	configPath := filepath.Dir(configFile)
	configName := filepath.Base(configFile)
	ext := filepath.Ext(configName)

	viper.SetConfigType(strings.TrimPrefix(ext, "."))
	viper.SetConfigName(strings.TrimSuffix(configName, ext))
	viper.AddConfigPath(configPath)
	viper.AddConfigPath(defaultConfigDir)

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	cfg := NewDefaultConfig()
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
