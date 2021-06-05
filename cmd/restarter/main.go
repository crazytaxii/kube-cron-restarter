package main

import (
	"os"

	"github.com/crazytaxii/kube-cron-restarter/cmd/restarter/app"
)

func main() {
	command := app.NewAutoRestarterCommand()
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
