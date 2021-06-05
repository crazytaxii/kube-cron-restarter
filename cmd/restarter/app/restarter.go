package app

import (
	"context"
	"fmt"
	"os"

	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app/config"
	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app/options"
	"github.com/crazytaxii/kube-cron-restarter/pkg/restarter"
	"github.com/spf13/cobra"
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

	controller, err := restarter.NewRestarterController(kubeClientset, cfg.ResyncPeriod,
		restarter.WithCronJobNamespace(cfg.CronJobNamespace),
		restarter.WithKubeCtlImage(cfg.KubeCtlImage),
	)
	if err != nil {
		return err
	}

	// Running a health check server
	go StartHealthzServer(cfg.HealthzPort)

	run := func(ctx context.Context) {
		// Launch 2 workers to process resources
		if err = controller.Run(workers, stopCh); err != nil {
			klog.Fatalf("Error running controller: %v", err)
		}
	}

	if cfg.LeaderElection.LeaderElect {

		hostname, err := os.Hostname()
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%s_%s", hostname, string(uuid.NewUUID()))
		rl, err := resourcelock.New(
			cfg.LeaderElection.ResourceLock,
			cfg.LeaderElection.ResourceNamespace,
			cfg.LeaderElection.ResourceName,
			cfg.LeaderClient.CoreV1(),
			cfg.LeaderClient.CoordinationV1(),
			resourcelock.ResourceLockConfig{
				Identity: id,
			},
		)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			<-stopCh
			klog.Info("Received termination, signaling shutdown")
			cancel()
		}()

		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:          rl,
			LeaseDuration: cfg.LeaderElection.LeaseDuration.Duration,
			RenewDeadline: cfg.LeaderElection.RenewDeadline.Duration,
			RetryPeriod:   cfg.LeaderElection.RetryPeriod.Duration,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: run,
				OnStoppedLeading: func() {
					klog.Error("Leader election lost")
				},
			},
			Name: cfg.LeaderElection.ResourceNamespace,
		})

		return nil
	}

	run(context.TODO())
	return nil
}
