package main

import (
	"os"

	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	command := app.NewAutoRestarterCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
