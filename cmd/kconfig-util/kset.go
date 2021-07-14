package main

import (
	"fmt"

	"github.com/jphx/kconfig/config"
)

type KsetOptions struct {
	config.KconfigOptions
}

var ksetOptions KsetOptions

func (o *KsetOptions) Usage() string {
	return "nickname [override-options]"
}

func (o *KsetOptions) Execute(args []string) error {
	commandProcessor = ksetProcessor
	commandName = "kset"

	switch len(args) {
	case 0:
		return fmt.Errorf("A kconfig nickname must be specified.")
	case 1:
		// Good
	default:
		return fmt.Errorf("Unrecognized positional argument provided after the kconfig nickname.")
	}

	return nil
}

//var ksetLogger = common.CreateLogger("kset")

func ksetProcessor(positionalArgs []string) {
	nickname := positionalArgs[0]
	kubeconfig, kubectlExecutable, overrides := config.CreateLocalKubectlConfigFile(nickname, &ksetOptions.KconfigOptions, true)

	// Print to standard output any shell operations that should be performed.
	fmt.Printf("export KUBECONFIG=%s\n", kubeconfig)

	kconfig := config.GetKconfig()
	if kconfig.Preferences.ChangePrompt == nil || *kconfig.Preferences.ChangePrompt {
		promptPrefix := nickname
		if overrides != "" && (kconfig.Preferences.ShowOverridesInPrompt == nil || *kconfig.Preferences.ShowOverridesInPrompt) {
			promptPrefix = fmt.Sprintf("%s[%s]", nickname, overrides)
		}

		// Emit a temporary shell variable that describes the prefix to use on the shell prompt.
		fmt.Printf("_KP=%s\n", promptPrefix)
	}

	// Set an environment variable used by the kubectl executable included with this package.
	fmt.Printf("export _KCONFIG_KUBECTL=%s\n", kubectlExecutable)

	// Set an environment variable that says what nickname is in effect.
	//fmt.Printf("export _KCONFIG_NICKNAME=%s\n", nickname)
}

func init() {
	_, err := parser.AddCommand("kset",
		"Create or update a session-local kubectl configuration file",
		"Creates a session-local kubectl configuration file whose current context is set to the "+
			"selected nickname, possibly modified by overriding options.  The KUBECONFIG "+
			"environment variable is set to a path that makes the session-local configuration file "+
			"active.",
		&ksetOptions)

	if err != nil {
		panic(fmt.Sprintf("Error adding command for parsing: %v", err))
	}
}
