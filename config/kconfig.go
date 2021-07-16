package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/shlex"
	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/jphx/kconfig/common"
)

var logger = common.CreateLogger("kconfig")

const kconfigContextName = "kconfig_context"

var kconfigTmpSessionDir = filepath.Join(os.TempDir(), "kconfig", "session")
var kconfigTmpNicknameDir = filepath.Join(os.TempDir(), "kconfig", "nicks")

// Kconfig describes the format of the ~/.kube/kconfig.yaml file.
type Kconfig struct {
	Preferences KconfigPreferences
	Nicknames   map[string]string
}

// KconfigPreferences describes the format of the kconfig.yaml file.
type KconfigPreferences struct {
	// DefaultKubectl give the name (with or without a path) of the kubectl executable to use if the
	// nickname definition doesn't explicitly provide one.  If not specified, the default is "kubectl".
	DefaultKubectl string `yaml:"default_kubectl,omitempty"`

	// ChangePrompt says whether or not the kconfig subcommand emits shell code to modify the PS1
	// shell variable.  If unspecified, the default is true.
	ChangePrompt *bool `yaml:"change_prompt,omitempty"`

	// ShowOverridesInPrompt says whether or not "overrides" are included in the shell prompt, when
	// it's being modified.  If unspecified, the default is true.
	ShowOverridesInPrompt *bool `yaml:"show_overrides_in_prompt,omitempty"`

	// ReadKaliasConfig says whether or not we'll look for the ~/.kube/kalias.txt file as a source
	// of nicknames.  The default is false, unless the ~/.kube/kconfig.yaml file doesn't exist, in
	// which cases it's true.
	ReadKaliasConfig bool `yaml:"read_kalias_config,omitempty"`

	// The default KUBECONFIG environment variable setting to be used.  If not specified, it
	// defaults to the empty string, which kubectl interprets as "~/.kube/config".
	BaseKubeconfig string `yaml:"base_kubeconfig,omitempty"`
}

// KconfigOptions describes the options that can appear in the kconfig nickname definition
type KconfigOptions struct {
	KubeConfig string `long:"kubeconfig" value-name:"FILE" description:"Path to the kubectl config file to use.  If not specified, the default is ~/.kube/config."`
	Context    string `long:"context" value-name:"NAME" description:"The name of the context to use from the kubectl config file.  If not specified, the default context is used."`
	Namespace  string `short:"n" long:"namespace" value-name:"NAME" description:"The namespace to use.  If not specified, the namespace associated the specified or default context is used."`
	User       string `long:"user" value-name:"NAME" description:"The user name to use.  If not specified, the user associated the specified or default context is used."`
}

func getHomeDirectory() string {
	homedir, err := os.UserHomeDir()
	if err != nil {
		common.RootLogger.Panic("Unable to get user's home directory: ", err)
	}

	return homedir
}

var cachedKconfig *Kconfig
var cachedKconfigError error

// GetKconfig fetches the configuration as describes in kconfig.yaml and possibly augmented with
// kalias.txt.  It's safe to call multiple times.  Only the first call with read and parse the
// files.  Subsequent calls will return cached results.
func GetKconfig() *Kconfig {
	//if cachedKconfigError != nil {
	//	return nil, cachedKconfigError
	//}

	if cachedKconfig != nil {
		return cachedKconfig
	}

	cachedKconfig, cachedKconfigError = readKconfig()
	if cachedKconfigError != nil {
		fmt.Fprintf(os.Stderr, "Error reading kconfig configuration file(s): %v\n", cachedKconfigError)
		os.Exit(1)
	}

	return cachedKconfig
}

func readKconfig() (*Kconfig, error) {
	kconfig := &Kconfig{
		Nicknames: make(map[string]string),
	}

	configFile, err := os.Open(filepath.Join(getHomeDirectory(), ".kube", "kconfig.yaml"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}

		// If the Kconfig file doesn't exist, maybe the Kalias file will exist.
		kconfig.Preferences.ReadKaliasConfig = true

	} else {
		// Read and parse the Kconfig file.
		err := yaml.NewDecoder(configFile).Decode(kconfig)
		configFile.Close()
		if err != nil {
			return nil, err
		}

		if kconfig.Nicknames == nil {
			kconfig.Nicknames = make(map[string]string)
		}

		//logger.Debugf("There are %d nicknames defined in kconfig.yaml.  Preferences are: %#v", len(kconfig.Nicknames), kconfig.Preferences)
	}

	if kconfig.Preferences.ReadKaliasConfig {
		logger.Debug("Merging contents of kalias.txt.")
		// We should merge the config we've read with the older kalias config file.
		configFile, err = os.Open(filepath.Join(getHomeDirectory(), ".kube", "kalias.txt"))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return kconfig, nil
			}
			return nil, err
		}

		nicknames := kconfig.Nicknames
		reader := bufio.NewReader(configFile)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				fmt.Fprintf(os.Stderr, "Ignoring error reading kalias definitions from \"%s\": %v\n", configFile.Name(), err)
				break
			}

			line = strings.TrimSpace(line)
			if len(line) == 0 {
				if err == io.EOF {
					break
				}
				continue
			}

			if line[0] == '#' {
				continue
			}

			equals := strings.IndexByte(line, '=')
			if equals == -1 {
				continue
			}

			nickname := line[0:equals]
			if _, isDefinedAlready := nicknames[nickname]; isDefinedAlready {
				// Definition from kconfig.yaml take precedence.
				continue
			}

			if equals == len(line)-1 {
				continue
			}

			defn := line[equals+1:]
			//logger.Debugw("Adding from kalias.", "nickname", nickname, "defn", defn)
			nicknames[nickname] = defn

			if err == io.EOF {
				break
			}
		}

		configFile.Close()
	} else {
		logger.Debug("Skipping merging of contents of kalias.txt.")
	}

	return kconfig, nil
}

func lookupKconfigNickname(nickname string) string {
	kconfig := GetKconfig()
	defn, exists := kconfig.Nicknames[nickname]
	if !exists {
		fmt.Fprintf(os.Stderr, "Nickname \"%s\" is not defined.\n", nickname)
		os.Exit(1)
	}

	return defn
}

func parseNicknameDefinition(definition string) (*KconfigOptions, string) {
	kubectlExecutable := GetKconfig().Preferences.DefaultKubectl
	if kubectlExecutable == "" {
		kubectlExecutable = "kubectl"
	}

	defnArgs, err := shlex.Split(definition)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing kconfig specification \"%s\": %v\n", definition, err)
		os.Exit(1)
	}

	if len(defnArgs) == 0 {
		fmt.Fprint(os.Stderr, "The kconfig specification is empty\n")
		os.Exit(1)
	}

	if len(defnArgs[0]) > 0 && defnArgs[0][0] != '-' {
		kubectlExecutable = defnArgs[0]
		defnArgs = defnArgs[1:]
	}

	var kconfigOptions KconfigOptions
	positionalArgs, err := flags.ParseArgs(&kconfigOptions, defnArgs)
	if err != nil {
		os.Exit(1)
	}

	if len(positionalArgs) > 0 {
		fmt.Fprintf(os.Stderr, "The kconfig specification has unrecognized arguments: %s\n", strings.Join(positionalArgs, " "))
		// In the above, shlex.Join() would be better, but the shlex library doesn't provide that function.
		os.Exit(1)
	}

	logger.Debugf("Parsed kconfig defn.  kubectl executable is \"%s\".  Options are: %#v", kubectlExecutable, kconfigOptions)
	return &kconfigOptions, kubectlExecutable
}

// CreateLocalKubectlConfigFile creates or replaces a local kubectl configuration file.  To figure
// out what information to put in the file, it uses the provided nickname and any override options.
// To create a session-local file, specify sessionFile as true.  In this case, the file name will be
// derived from the current KUBECONFIG environment variable, or if one isn't named that, created
// with a random name.  When creating a non-session-local file, specify kconfigOptions as nil, since
// overrides are not allowed in that case.  If an error occurs, the process is exited with an error
// message.  On success, the new value to be used as the KUBECONFIG environment variable is
// returned, as well as the kubectl executable that should be used for this nickname, and a short
// description of any overrides used (in case the caller want that information for the shell
// prompt).
func CreateLocalKubectlConfigFile(nickname string, kconfigOptions *KconfigOptions, sessionFile bool) (string, string, string) {
	if !sessionFile {
		if kconfigOptions != nil {
			panic("Call to CreateLocalKubectlConfigFile specified a non-nil KconfigOptions")
		}
		kconfigOptions = &KconfigOptions{} // So we don't have keep checking for nil
	}

	defn := lookupKconfigNickname(nickname)
	logger.Debugf("The definition is nickname \"%s\" is: %s", nickname, defn)

	// Parse the nickname's definition
	nicknameOptions, kubectlExecutable := parseNicknameDefinition(defn)
	var overrides []string

	// We're going to need the current value of the KUBECONFIG environment variable later, so fetch
	// it before we change it.
	kubeconfigEnvVar, kubeconfigEnvVarIsSet := os.LookupEnv("KUBECONFIG")

	// When reading the kube config using ReadKubeConfig(), the library it calls will read and use
	// the KUBECONFIG env var.  We'd like it to use a "fresh" value that doesn't include any
	// session-local kubectl config file or a temporary search path that's related to the
	// session-local file.  We therefore set it here for this process so it gets used during the
	// parsing.  If there's an override --kubeconfig option, use that.  Otherwise if the nickname
	// definition has the --kubeconfig option, use that.  Otherwise use an empty value to ask for
	// the default search path.
	searchPath := GetKconfig().Preferences.BaseKubeconfig
	if nicknameOptions.KubeConfig != "" {
		searchPath = nicknameOptions.KubeConfig
	}
	if kconfigOptions.KubeConfig != "" {
		searchPath = kconfigOptions.KubeConfig
	}
	logger.Debugf("Search path for reading config is: %s", searchPath)
	err := os.Setenv("KUBECONFIG", searchPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to update the KUBECONFIG environment variable: %v\n", err)
		os.Exit(1)
	}

	// Read the kubectl config information that establishes the configuration we're working with.
	kubeconfig := ReadKubeConfig()

	// Restore the KUBECONFIG environment variable, in case it's important to the caller.
	if !kubeconfigEnvVarIsSet {
		err = os.Unsetenv("KUBECONFIG")
	} else {
		err = os.Setenv("KUBECONFIG", kubeconfigEnvVar)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error restoring the KUBECONFIG environment variable: %v\n", err)
		os.Exit(1)
	}

	// Figure out what kubectl context we should refer to.
	baseContext := kubeconfig.CurrentContext
	logger.Debugf("Current context from base is: %s", baseContext)
	if nicknameOptions.Context != "" {
		baseContext = nicknameOptions.Context
	}
	if kconfigOptions.Context != "" {
		baseContext = kconfigOptions.Context
	}
	logger.Debugf("Context after overriding is: %s", baseContext)

	if baseContext == "" {
		fmt.Fprintf(os.Stderr, "There is no current context in search path: %s\n", searchPath)
		os.Exit(1)
	}

	contextDefn, exists := kubeconfig.Contexts[baseContext]
	if !exists {
		fmt.Fprintf(os.Stderr, "Context \"%s\" doesn't exist.\n", baseContext)
		os.Exit(1)
	}

	// See if our new config file can be a simple "current-context" entry or if it must define
	// a new context so that namespace or user can be overridden.
	needNewContext := nicknameOptions.Namespace != "" || nicknameOptions.User != "" ||
		kconfigOptions.Namespace != "" || kconfigOptions.User != ""
	logger.Debugf("Need new context?: %v", needNewContext)

	// Create the content for the session-local kubectl config file
	newConfigFileContent := clientcmdapi.NewConfig()
	if !needNewContext {
		newConfigFileContent.CurrentContext = baseContext

	} else {
		// Copy the referenced context to start with
		newContext := contextDefn.DeepCopy()
		// So our change doesn't get written back to the file where the context is defined:
		newContext.LocationOfOrigin = ""
		logger.Debugf("Initial context: %#v", newContext)

		// Set the namespace
		if nicknameOptions.Namespace != "" {
			newContext.Namespace = nicknameOptions.Namespace
		}
		if kconfigOptions.Namespace != "" {
			newContext.Namespace = kconfigOptions.Namespace
			overrides = append(overrides, fmt.Sprintf("ns=%s", kconfigOptions.Namespace))
		}

		// Set the user
		if nicknameOptions.User != "" {
			newContext.AuthInfo = nicknameOptions.User
		}
		if kconfigOptions.User != "" {
			newContext.AuthInfo = kconfigOptions.User
			overrides = append(overrides, fmt.Sprintf("u=%s", kconfigOptions.User))
		}
		logger.Debugf("Context after overrides: %#v", newContext)

		// Add it to the config and make it the current context
		newConfigFileContent.CurrentContext = kconfigContextName
		newConfigFileContent.Contexts[kconfigContextName] = newContext
	}

	parentDir := kconfigTmpNicknameDir
	fileIsEmpty := false
	localConfigFilename := filepath.Join(kconfigTmpNicknameDir, fmt.Sprintf("%s.yaml", nickname))
	if sessionFile {
		parentDir = kconfigTmpSessionDir
		localConfigFilename = GetExistingSessionLocalFilename(kubeconfigEnvVar)
	}

	err = os.MkdirAll(parentDir, os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create temporary directory \"%s\" for local kubectl config file: %v\n", parentDir, err)
		os.Exit(1)
	}

	if localConfigFilename == "" {
		// Must be a session file that we need to create.
		localConfigFilename = createSessionKubeconfigFile(parentDir)
		// This actually creates an empty file.  Remember that so we can clean it up if we encounter
		// a failure before writing the file.
		fileIsEmpty = true
	}

	// Create or replace the current session-local kubectl config file.
	configAccess := &clientcmd.PathOptions{
		GlobalFile:   localConfigFilename,
		EnvVar:       "",
		LoadingRules: clientcmd.NewDefaultClientConfigLoadingRules(),
	}

	// Suppress any warning that might result of a missing target file.
	configAccess.LoadingRules.WarnIfAllMissing = false

	err = clientcmd.ModifyConfig(configAccess, *newConfigFileContent, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating the session-local kubectl configuration file \"%s\": %v\n", localConfigFilename, err)
		if fileIsEmpty {
			os.Remove(localConfigFilename)
		}
		os.Exit(1)
	}

	verb := "Replaced"
	if fileIsEmpty {
		verb = "Created"
	}
	logger.Debugf("%s local config file: %s", verb, localConfigFilename)

	// Work out the new KUBECONFIG environment variable value to use.

	if searchPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to find user's home directory: %v\n", err)
			if fileIsEmpty {
				// It isn't empty anymore, but the KUBECONFIG env var doesn't name it, so it's
				// effectively orphaned.
				os.Remove(localConfigFilename)
			}
			os.Exit(1)
		}
		searchPath = filepath.Join(homeDir, ".kube", "config")
	}

	newKubeconfigEnvVar := fmt.Sprintf("%s%c%s", localConfigFilename, os.PathListSeparator, searchPath)
	return newKubeconfigEnvVar, kubectlExecutable, strings.Join(overrides, ",")
}

// GetExistingSessionLocalFilename parses the passed value, which is interpreted as a KUBECONFIG
// value.  If the first entry in the search path refers to a session-local kubectl config file, its
// name is returned.  Otherwise an empty string is returned.
func GetExistingSessionLocalFilename(kubeconfigEnvVar string) string {
	//kubeconfigEnvVar := os.Getenv("KUBECONFIG")
	logger.Debugf("Fetched KUBECONFIG of: %s", kubeconfigEnvVar)
	if kubeconfigEnvVar == "" || !strings.HasPrefix(kubeconfigEnvVar, kconfigTmpSessionDir) {
		logger.Debug("Doesn't contain a session config file name")
		return ""
	}

	filename := kubeconfigEnvVar
	pathSeparator := strings.IndexByte(filename, os.PathListSeparator)
	if pathSeparator != -1 {
		filename = kubeconfigEnvVar[0:pathSeparator]
	}
	logger.Debugf("Contains filename: %s", filename)
	return filename
}

func createSessionKubeconfigFile(kconfigTmpDir string) string {
	sessionKubeconfigFile, err := os.CreateTemp(kconfigTmpDir, "*.yaml")
	if err != nil {
		if sessionKubeconfigFile != nil {
			fmt.Fprintf(os.Stderr, "Unable to create session-local temporary kubectl config file \"%s\": %v\n", sessionKubeconfigFile.Name(), err)
		} else {
			fmt.Fprintf(os.Stderr, "Unable to create session-local temporary kubectl config file: %v\n", err)
		}
		os.Exit(1)
	}

	sessionKubeconfigFile.Close()
	return sessionKubeconfigFile.Name()
}
