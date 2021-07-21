package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jphx/kconfig/config"
)

type koffCommandOptions struct {
}

var koffOptions koffCommandOptions

func (o *koffCommandOptions) Usage() string {
	return ""
}

func (o *koffCommandOptions) Execute(args []string) error {
	commandProcessor = koffProcessor
	commandName = "koff"

	if len(args) > 0 {
		return fmt.Errorf("Unrecognized positional arguments provided.")
	}

	return nil
}

func koffProcessor(positionalArgs []string) {
	kubeconfigEnvVar := os.Getenv("KUBECONFIG")
	if kubeconfigEnvVar == "" {
		return
	}

	localConfigFilename := config.GetExistingSessionLocalFilename(kubeconfigEnvVar)
	if localConfigFilename != "" {
		err := os.Remove(localConfigFilename)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "Error removing session-local kubectl configuration file: %v\n", err)
		}
	}

	baseKubeconfig := config.GetKconfig().Preferences.BaseKubeconfig
	if baseKubeconfig != "" {
		fmt.Printf("export KUBECONFIG=%s\n", baseKubeconfig)
	} else {
		fmt.Println("unset KUBECONFIG")
	}

	// The koff shell function will unset the _KCONFIG_KUBECTL environment variable.
}

func init() {
	_, err := parser.AddCommand("koff",
		"Clean up session-local kubectl config file",
		"Called by koff shell function to remove any session-local kubectl config file and to "+
			"restore the KUBECONFIG env var to it's \"normal\" value.",
		&koffOptions)

	if err != nil {
		panic(fmt.Sprintf("Error adding command for parsing: %v", err))
	}
}
