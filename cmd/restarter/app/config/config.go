package config

import (
	"path/filepath"
	"time"

	"github.com/crazytaxii/kube-cron-restarter/pkg/restarter"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/homedir"
	componentBaseConfig "k8s.io/component-base/config"
)

const (
	defaultKubeConfigDir = ".kube/config"
)

type (
	AutoRestarterConfig struct {
		HealthzPort    int                                             // the port health server listening on
		LeaderElection componentBaseConfig.LeaderElectionConfiguration // config of leader election client
		ResyncPeriod   time.Duration
		LeaderClient   clientset.Interface
		EventRecorder  record.EventRecorder
		restarter.ControllerOptions
	}
)

func BuildKubeConfig() (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	return clientcmd.BuildConfigFromFlags("", filepath.Join(homedir.HomeDir(), defaultKubeConfigDir))
}
