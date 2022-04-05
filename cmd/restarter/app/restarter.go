package app

import (
	"context"
	"fmt"
	"os"

	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app/config"
	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app/options"
	"github.com/crazytaxii/kube-cron-restarter/pkg/controller"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

const workers = 2

func NewAutoRestarterCommand() *cobra.Command {
	opts := options.NewAutoRestarterOptions()

	cmd := &cobra.Command{
		Use:  "cron-restarter",
		Long: `The auto restarter is a controller restarts specific pod regularly`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := opts.Config()
			if err != nil {
				klog.Error(err)
				os.Exit(1)
			}
			if err := Run(cfg, signals.SetupSignalHandler()); err != nil {
				klog.Error(err)
				os.Exit(1)
			}
		},
	}

	opts.BindFlags(cmd)
	return cmd
}

func Run(cfg *config.AutoRestarterConfig, stopCh <-chan struct{}) error {
	kubeConfig, err := config.BuildKubeConfig()
	if err != nil {
		return err
	}
	kubeClientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	controller, err := controller.NewRestarterController(kubeClientset, cfg.ResyncPeriod,
		cfg.ControllerOptions,
	)
	if err != nil {
		return err
	}

	run := func(ctx context.Context) {
		// Running a health check server
		go StartHealthzServer(cfg.HealthzPort)
		// Launch 2 workers to process resources
		if err := controller.Run(workers, stopCh); err != nil {
			klog.Fatalf("Error running controller: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-stopCh
		klog.Info("Received termination, signaling shutdown")
		cancel()
	}()

	if cfg.LeaderElection.LeaderElect {
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%s_%s", hostname, string(uuid.NewUUID()))
		lock := &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      cfg.LeaderElection.ResourceName,
				Namespace: cfg.LeaderElection.ResourceNamespace,
			},
			Client: kubeClientset.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: id,
			},
		}

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   cfg.LeaderElection.LeaseDuration,
			RenewDeadline:   cfg.LeaderElection.RenewDeadline,
			RetryPeriod:     cfg.LeaderElection.RetryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: run,
				OnStoppedLeading: func() {
					klog.Errorf("Leader election lost: %d", id)
				},
				OnNewLeader: func(identity string) {
					// we're notified when new leader elected
					if identity == id {
						// I just got the lock
						return
					}
					klog.Infof("new leader elected: %s", identity)
				},
			},
			Name: cfg.LeaderElection.ResourceNamespace,
		})
		return nil
	}

	run(ctx)
	return nil
}
