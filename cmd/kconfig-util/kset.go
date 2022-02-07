package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jphx/kconfig/common"
	"github.com/jphx/kconfig/config"
)

const ksetEnvVarDelimiter = "\x1F"

type ksetCommandOptions struct {
	config.KconfigOptions
}

var ksetOptions ksetCommandOptions

func (o *ksetCommandOptions) Usage() string {
	return "[nickname|-] [override-options]"
}

func (o *ksetCommandOptions) Execute(args []string) error {
	commandProcessor = ksetProcessor
	commandName = "kset"

	switch len(args) {
	case 0:
		if os.Getenv("_KCONFIG_KSET") == "" {
			return fmt.Errorf("A kconfig nickname must be specified unless one is already in effect.")
		}

	case 1:
		// Good
		nickname := args[0]
		if nickname == "-" && os.Getenv("_KCONFIG_OLDKSET") == "" {
			return fmt.Errorf("A kconfig nickname of \"-\" can only be used when a previous kconfig environment is in effect.")
		}

	default:
		return fmt.Errorf("Unrecognized positional argument provided after the kconfig nickname.")
	}

	return nil
}

var ksetLogger = common.CreateLogger("kset")

func ksetProcessor(positionalArgs []string) {
	var nickname string
	if len(positionalArgs) == 0 {
		nickname = getNicknameFromKsetArgs(os.Getenv("_KCONFIG_KSET"))
		if nickname == "" {
			fmt.Fprintln(os.Stderr, "A kconfig nickname must be specified unless one is already in effect.")
			os.Exit(1)
		}
		ksetLogger.Debugf("Processing missing nickname in kset.  Deduced nickname \"%s\".", nickname)

	} else {
		nickname = positionalArgs[0]
		if nickname == "-" {
			// This can only occur when the kset command has a nickname AND arguments, meaning the
			// user only wants to use the previous nickname, and not the previous arguments as well.
			// A plain "kset -" would be handled in main.go and transformed into (essentially)
			// "kset $_KCONFIG_OLDKSET" before the arguments are parsed.  So we're dealing with
			// something like "kset - -n xxx" instead, where only the previous nickname is used.
			nickname = getNicknameFromKsetArgs(os.Getenv("_KCONFIG_OLDKSET"))
			if nickname == "" {
				fmt.Fprintln(os.Stderr, "A kconfig nickname of \"-\" can only be used when a previous kconfig environment is in effect.")
				os.Exit(1)
			}

			ksetLogger.Debugf("Processing nickname of \"-\" in kset.  Deduced nickname \"%s\".", nickname)
		}
	}

	createResults := config.CreateLocalKubectlConfigFile(nickname, &ksetOptions.KconfigOptions, true)

	// Print to standard output any shell operations that should be performed.
	fmt.Printf("export KUBECONFIG=%s\n", createResults.NewKubeconfigEnvVar)

	// If the user is using Teleport, see if they've asked for us to set the TELEPORT_PROXY
	// environment variable that Teleport uses when it proxies a Kubernetes connection.
	if createResults.TeleportProxyEnvVar != "" {
		fmt.Printf("export TELEPORT_PROXY=%s\n", createResults.TeleportProxyEnvVar)
	}

	kconfig := config.GetKconfig()
	if kconfig.Preferences.ChangePrompt == nil || *kconfig.Preferences.ChangePrompt {
		promptPrefix := nickname
		if createResults.OverridesDescription != "" && (kconfig.Preferences.ShowOverridesInPrompt == nil || *kconfig.Preferences.ShowOverridesInPrompt) {
			if kconfig.Preferences.AlwaysShowNamespaceInPrompt && !strings.Contains(createResults.OverridesDescription, "ns=") {
				createResults.OverridesDescription = fmt.Sprintf("ns=%s,%s", createResults.ContextNamespace, createResults.OverridesDescription)
			}
			promptPrefix = fmt.Sprintf("%s[%s]", nickname, createResults.OverridesDescription)

		} else if kconfig.Preferences.AlwaysShowNamespaceInPrompt {
			promptPrefix = fmt.Sprintf("%s[ns=%s]", nickname, createResults.ContextNamespace)
		}

		// Emit a temporary shell variable that describes the prefix to use on the shell prompt.
		fmt.Printf("_KP=%s\n", promptPrefix)
	}

	// Set an environment variable used by the kubectl executable included with this package.
	fmt.Printf("export _KCONFIG_KUBECTL=%s\n", createResults.KubectlExecutable)

	// Figure out the description of the new kset environment.
	ksetDescription := createKsetArgs(nickname, &ksetOptions.KconfigOptions)

	// Transfer the description of the most-recent kset environment to the _KCONFIG_OLDKSET env var.
	previousKset := os.Getenv("_KCONFIG_KSET")
	if previousKset != "" && previousKset != ksetDescription {
		fmt.Println("export _KCONFIG_OLDKSET=\"$_KCONFIG_KSET\"")
	}

	// Set an environment variable that says what the current kset request is.  We might use this
	// later, once it gets transferred to the _KCONFIG_OLDKSET environment variable, when processing
	// a "kset -" command, which says to switch the last kset environment.
	fmt.Printf("export _KCONFIG_KSET=\"%s\"\n", ksetDescription)
}

// createKsetArgs creates a string that describes the kset environment, the nickname and any
// overrides.  We'd like to properly quote the values in this string as a shell would so that we can
// parse them again later, but sadly the github.com/google/shlex library that we use for parsing a
// quoted string doesn't support quoting a string.  So instead we delimit the fields with a simple
// blank character, *unless* a blank appears in any of the values.  In that case, we use a delimiter
// that should not appear in the string, namely the "unit separator" ASCII/Unicode control code, 0x1F.
func createKsetArgs(nickname string, kconfigOptions *config.KconfigOptions) string {
	// Fast path for common case when no override options are specified.
	if kconfigOptions.KubeConfig == "" && kconfigOptions.Context == "" &&
		kconfigOptions.Namespace == "" && kconfigOptions.User == "" &&
		kconfigOptions.TeleportProxy == "" {
		return nickname
	}

	var args []string
	args = append(args, nickname)
	if kconfigOptions.KubeConfig != "" {
		args = append(args, "--kubeconfig", kconfigOptions.KubeConfig)
	}
	if kconfigOptions.Context != "" {
		args = append(args, "--context", kconfigOptions.Context)
	}
	if kconfigOptions.Namespace != "" {
		args = append(args, "-n", kconfigOptions.Namespace)
	}
	if kconfigOptions.User != "" {
		args = append(args, "--user", kconfigOptions.User)
	}
	if kconfigOptions.TeleportProxy != "" {
		args = append(args, "--teleport-proxy", kconfigOptions.TeleportProxy)
	}

	delimiter := " "
	if strings.Contains(nickname, " ") || strings.Contains(kconfigOptions.KubeConfig, " ") ||
		strings.Contains(kconfigOptions.Context, " ") ||
		strings.Contains(kconfigOptions.Namespace, " ") ||
		strings.Contains(kconfigOptions.User, " ") ||
		strings.Contains(kconfigOptions.TeleportProxy, " ") {
		delimiter = ksetEnvVarDelimiter
	}

	return strings.Join(args, delimiter)
}

func getNicknameFromKsetArgs(ksetEnvValue string) string {
	ksetArgs := getArgsFromKsetArgs(ksetEnvValue)
	if len(ksetArgs) == 0 {
		return ""
	}

	return ksetArgs[0]
}

func getArgsFromKsetArgs(ksetEnvValue string) []string {
	delimiter := " "
	if strings.Contains(ksetEnvValue, ksetEnvVarDelimiter) {
		delimiter = ksetEnvVarDelimiter
	}
	return strings.Split(ksetEnvValue, delimiter)
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
