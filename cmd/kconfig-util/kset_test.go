package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/jphx/kconfig/common"
	"github.com/jphx/kconfig/config"
)

const kconfigUtilCommand = "../../bin/kconfig-util"

type TestCase struct {
	Name                  string
	Preferences           config.KconfigPreferences
	CopyKconfigYaml       bool
	CopyKaliasTxt         bool
	Arguments             []string
	KsetEnvVar            string
	OldKsetEnvVar         string
	ExpectError           string
	ExpectKubeconfig      string
	ExpectKubectlExe      string
	ExpectPrompt          string
	ExpectLocalConfigFile string
	ExpectTeleportProxy   string
}

var casesToTest = []TestCase{
	{
		Name:        "Bad short option",
		Arguments:   []string{"dev", "-f"},
		ExpectError: "unknown flag .f.",
	},
	{
		Name:        "Bad long option",
		Arguments:   []string{"dev", "--bad-option"},
		ExpectError: "unknown flag .bad-option.",
	},
	{
		Name:        "Bad nickname",
		Arguments:   []string{"doesnt-exist"},
		ExpectError: "Nickname \"doesnt-exist\" is not defined.",
	},
	{
		Name:                  "Simple nickname",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev",
		ExpectLocalConfigFile: "1",
	},
	{
		Name:                  "Nickname with just kconfig",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       false,
		CopyKaliasTxt:         true,
		Arguments:             []string{"devfromkalias"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "devfromkalias",
		ExpectLocalConfigFile: "1",
	},
	{
		Name:            "Nickname from kalias not allowed",
		Preferences:     config.KconfigPreferences{},
		CopyKconfigYaml: true,
		CopyKaliasTxt:   true,
		Arguments:       []string{"devfromkalias"},
		ExpectError:     "Nickname \"devfromkalias\" is not defined.",
	},
	{
		Name: "Nickname from kalias allowed",
		Preferences: config.KconfigPreferences{
			ReadKaliasConfig: true,
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         true,
		Arguments:             []string{"devfromkalias"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "devfromkalias",
		ExpectLocalConfigFile: "1",
	},
	{
		Name:                  "Nickname has namespace",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-namespace"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-namespace",
		ExpectLocalConfigFile: "2",
	},
	{
		Name: "Nickname has context with no namespace",
		Preferences: config.KconfigPreferences{
			AlwaysShowNamespaceInPrompt: true,
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-no-namespace-in-context"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-no-namespace-in-context[ns=default]",
		ExpectLocalConfigFile: "1.1",
	},
	{
		Name:                  "Nickname has user",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-user"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-user",
		ExpectLocalConfigFile: "3",
	},
	{
		Name: "Nickname has user, show namespace in prompt",
		Preferences: config.KconfigPreferences{
			AlwaysShowNamespaceInPrompt: true,
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-user"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-user[ns=devnamespace1]",
		ExpectLocalConfigFile: "3",
	},
	{
		Name:                  "Nickname has user and namespace",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-namespace-user"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-namespace-user",
		ExpectLocalConfigFile: "4",
	},
	{
		Name: "Nickname has user and namespace, show namespace in prompt",
		Preferences: config.KconfigPreferences{
			AlwaysShowNamespaceInPrompt: true,
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-namespace-user"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-namespace-user[ns=namespace-override]",
		ExpectLocalConfigFile: "4",
	},
	{
		Name: "Nickname has user and namespace, show namespace, but no prompt changes",
		Preferences: config.KconfigPreferences{
			ChangePrompt:                GetBoolPtr(false),
			AlwaysShowNamespaceInPrompt: true,
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-namespace-user"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "",
		ExpectLocalConfigFile: "4",
	},
	{
		Name:            "Nickname has invalid option",
		Preferences:     config.KconfigPreferences{},
		CopyKconfigYaml: true,
		CopyKaliasTxt:   false,
		Arguments:       []string{"bad-option"},
		ExpectError:     "unknown flag .bad-option.",
	},
	{
		Name:                  "Nickname has executable",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-with-executable"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl-99",
		ExpectPrompt:          "dev-with-executable",
		ExpectLocalConfigFile: "1",
	},
	{
		Name: "Default executable",
		Preferences: config.KconfigPreferences{
			DefaultKubectl: "kubectl-default",
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl-default",
		ExpectPrompt:          "dev",
		ExpectLocalConfigFile: "1",
	},
	{
		Name:                  "Override namespace on command",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev", "-n", "namespace-override"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev[ns=namespace-override]",
		ExpectLocalConfigFile: "2",
	},
	{
		Name:                  "Override kconfig namespace on command",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-namespace", "-n", "namespace-override"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-namespace[ns=namespace-override]",
		ExpectLocalConfigFile: "2",
	},
	{
		Name:                  "Override kconfig namespace on command, with no nickname",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"-n", "namespace-override"},
		KsetEnvVar:            "dev-namespace -n other-namespace",
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-namespace[ns=namespace-override]",
		ExpectLocalConfigFile: "2",
	},
	{
		Name:                  "Override kconfig namespace on command, with dash for nickname",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"-", "-n", "namespace-override"},
		OldKsetEnvVar:         "dev-namespace -n other-namespace",
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-namespace[ns=namespace-override]",
		ExpectLocalConfigFile: "2",
	},
	{
		Name:                  "Override user on command",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev", "--user", "devuser2"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev[u=devuser2]",
		ExpectLocalConfigFile: "3",
	},
	{
		Name:                  "Override namespace and user on command",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev", "-n", "namespace-override", "--user", "devuser2"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev[ns=namespace-override,u=devuser2]",
		ExpectLocalConfigFile: "4",
	},
	{
		Name: "Override user on command no prompt change",
		Preferences: config.KconfigPreferences{
			ChangePrompt: GetBoolPtr(false),
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev", "--user", "devuser2"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "",
		ExpectLocalConfigFile: "3",
	},
	{
		Name: "Override user on command no prompt overrides",
		Preferences: config.KconfigPreferences{
			ShowOverridesInPrompt: GetBoolPtr(false),
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev", "--user", "devuser2"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev",
		ExpectLocalConfigFile: "3",
	},
	{
		Name:                  "Override kubeconfig",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-with-kubeconfig"},
		ExpectKubeconfig:      ".kube/testing.config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-with-kubeconfig",
		ExpectLocalConfigFile: "5",
	},
	{
		Name: "Override user on command also show namespace in prompt",
		Preferences: config.KconfigPreferences{
			AlwaysShowNamespaceInPrompt: true,
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev", "--user", "devuser2"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev[ns=devnamespace1,u=devuser2]",
		ExpectLocalConfigFile: "3",
	},
	{
		Name: "Override user on command only show namespace in prompt",
		Preferences: config.KconfigPreferences{
			ShowOverridesInPrompt:       GetBoolPtr(false),
			AlwaysShowNamespaceInPrompt: true,
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev", "--user", "devuser2"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev[ns=devnamespace1]",
		ExpectLocalConfigFile: "3",
	},
	{
		Name:                  "Using just a nickname of dash",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"-"},
		OldKsetEnvVar:         "dev-namespace -n namespace-override",
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-namespace[ns=namespace-override]",
		ExpectLocalConfigFile: "2",
	},
	{
		Name: "Different default kubectl config",
		Preferences: config.KconfigPreferences{
			BaseKubeconfig: "$HOME/.kube/testing.config",
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-assume-testing"},
		ExpectKubeconfig:      ".kube/testing.config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-assume-testing",
		ExpectLocalConfigFile: "5",
	},
	{
		Name: "Different default kubectl config multipath",
		Preferences: config.KconfigPreferences{
			BaseKubeconfig: "$HOME/.kube/testing.missing:$HOME/.kube/testing.config",
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-assume-testing"},
		ExpectKubeconfig:      ".kube/testing.missing:.kube/testing.config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-assume-testing",
		ExpectLocalConfigFile: "5",
	},
	{
		Name: "Different default kubectl config with config override",
		Preferences: config.KconfigPreferences{
			BaseKubeconfig: "$HOME/.kube/missing.config",
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-with-kubeconfig"},
		ExpectKubeconfig:      ".kube/testing.config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-with-kubeconfig",
		ExpectLocalConfigFile: "5",
	},
	{
		Name: "Different default kubectl config with extra",
		Preferences: config.KconfigPreferences{
			BaseKubeconfig: "$HOME/.kube/testing.missing:$HOME/.kube/missing.config",
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-with-kubeconfig"},
		ExpectKubeconfig:      ".kube/testing.config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-with-kubeconfig",
		ExpectLocalConfigFile: "5",
	},
	{
		Name: "Different kubectl config with context",
		Preferences: config.KconfigPreferences{
			BaseKubeconfig: "$HOME/.kube/missing.config",
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-with-kubeconfig-and-context"},
		ExpectKubeconfig:      ".kube/testing.config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-with-kubeconfig-and-context",
		ExpectLocalConfigFile: "6",
	},
	{
		Name: "Different kubectl config with namespace",
		Preferences: config.KconfigPreferences{
			BaseKubeconfig: "$HOME/.kube/missing.config",
		},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-with-kubeconfig-and-context-and-namespace"},
		ExpectKubeconfig:      ".kube/testing.config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-with-kubeconfig-and-context-and-namespace",
		ExpectLocalConfigFile: "7",
	},
	{
		Name:                  "Simple nickname with Teleport proxy",
		Preferences:           config.KconfigPreferences{},
		CopyKconfigYaml:       true,
		CopyKaliasTxt:         false,
		Arguments:             []string{"dev-with-teleport-proxy"},
		ExpectKubeconfig:      ".kube/config",
		ExpectKubectlExe:      "kubectl",
		ExpectPrompt:          "dev-with-teleport-proxy",
		ExpectLocalConfigFile: "1",
		ExpectTeleportProxy:   "tport-proxy1",
	},
}

var testHomeDir string

func TestMain(m *testing.M) {
	// Set the HOME environment variable to the testdata/home directory, so that we can control
	// what kconfig.yaml and kalias.txt files are there.
	var err error
	testHomeDir, err = filepath.Abs(filepath.Join("testdata", "home"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error calculating absolute path of test home dir: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Setting HOME env var to test home dir: %s\n", testHomeDir)
	err = os.Setenv("HOME", testHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting HOME env var to test home dir: %v\n", err)
		os.Exit(1)
	}

	// Enable debug-level logging
	common.LoggingLevel.SetLevel(zap.DebugLevel)

	// Now launch the tests
	os.Exit(m.Run())
}

var extractKubeconfigEnvVar = regexp.MustCompile(`(?m)^export KUBECONFIG=(.*)$`)
var extractTeleportProxyEnvVar = regexp.MustCompile(`(?m)^export TELEPORT_PROXY=(.*)$`)
var extractKubectlExe = regexp.MustCompile(`(?m)^export _KCONFIG_KUBECTL=(.*)$`)
var extractPrompt = regexp.MustCompile(`(?m)^_KP=(.*)$`)

func TestKsetResults(t *testing.T) {
	// Create a special /tmp directory for the files that are produced, to avoid the same /tmp
	// directories that might be in use for real uses of the utility.
	workarea := t.TempDir()

	fmt.Printf("Setting TMPDIR env var to test work area: %s\n", workarea)
	err := os.Setenv("TMPDIR", workarea)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting TMPDIR env var to test work area: %v\n", err)
		os.Exit(1)
	}

	unscrubbedEnvVars := os.Environ()
	environmentVars := unscrubbedEnvVars[:0] // Slice that shared underlying array

	// Scrub some env vars from array, so they can't affect the tests
	for _, value := range unscrubbedEnvVars {
		if !strings.HasPrefix(value, "_KCONFIG_KSET") && !strings.HasPrefix(value, "_KCONFIG_OLDKSET") {
			environmentVars = append(environmentVars, value)
		}
	}

	for _, testCase := range casesToTest {
		t.Run(testCase.Name, func(t *testing.T) {
			// Initialize the files in the home directory appropriately.
			if testCase.CopyKconfigYaml {
				err = copyConfigFile(t, "kconfig.yaml", &testCase.Preferences)
				if err != nil {
					t.Errorf("Error copying \"kconfig.yaml\": %v", err)
					return
				}
			}
			if testCase.CopyKaliasTxt {
				err = copyConfigFile(t, "kalias.txt", nil)
				if err != nil {
					t.Errorf("Error copying \"kalias.txt\": %v", err)
					return
				}
			}

			argv := []string{
				kconfigUtilCommand,
				//"--debug",
				"kset",
			}
			argv = append(argv, testCase.Arguments...)

			envVarsForTest := environmentVars
			if testCase.OldKsetEnvVar != "" {
				envVarsForTest = append(envVarsForTest, fmt.Sprintf("_KCONFIG_OLDKSET=%s", testCase.OldKsetEnvVar))
			}
			if testCase.KsetEnvVar != "" {
				t.Log("Assigning _KCONFIG_KSET env var")
				envVarsForTest = append(envVarsForTest, fmt.Sprintf("_KCONFIG_KSET=%s", testCase.KsetEnvVar))
			}

			cmd := exec.Command(argv[0], argv[1:]...)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			cmd.Env = envVarsForTest
			outputBytes, err := cmd.Output()
			if err != nil {
				if testCase.ExpectError == "" {
					t.Logf("kset command \"%s\" failed, but was expected to succeed: %v", strings.Join(argv, " "), err)
					t.Log("kset sent to standard error:")
					t.Log(stderr.String())
					t.Fail()
					return
				}

				expr, err := regexp.Compile(testCase.ExpectError)
				if err != nil {
					t.Errorf("Regular expression \"%s\" failed to compile: %v", testCase.ExpectError, err)
					return
				}

				if !expr.MatchString(stderr.String()) {
					t.Errorf("Error message should match \"%s\", but it doesn't.  It's: %s", testCase.ExpectError, stderr.String())
					return
				}
				return
			}

			output := string(outputBytes)

			if testCase.ExpectError != "" {
				t.Logf("kset command \"%s\" succeeded, but was expected to fail with error: %s", strings.Join(argv, " "), testCase.ExpectError)
				t.Logf("Command stdout was: %s", output)
				t.Logf("Command stderr was: %s", stderr.String())
				t.Fail()
				return
			}

			localKubectlConfigFile, failed := verifyKubeconfigEnvVar(t, output, &testCase)
			if failed {
				return
			}

			if verifyTeleportProxyEnvVar(t, output, &testCase) {
				return
			}

			if verifyKubectlExe(t, output, &testCase) {
				return
			}

			if verifyPrompt(t, output, &testCase) {
				return
			}

			if verifyLocalKubectlConfigFile(t, localKubectlConfigFile, &testCase) {
				return
			}
		})
	}
}

func verifyKubeconfigEnvVar(t *testing.T, output string, testCase *TestCase) (string, bool) {
	match := extractKubeconfigEnvVar.FindStringSubmatch(output)
	if match == nil {
		t.Log("Couldn't find the KUBECONFIG environment variable in the output.")
		t.Logf("output: %s", output)
		t.Fail()
		return "", true
	}
	value := match[1]
	//t.Logf("Actual   KUBECONFIG: %s", value)
	//t.Logf("Expected KUBECONFIG: %s", testCase.ExpectKubeconfig)
	searchPath := strings.Split(value, string(os.PathListSeparator))
	expectedPath := strings.Split(testCase.ExpectKubeconfig, string(os.PathListSeparator))
	if len(expectedPath)+1 != len(searchPath) {
		t.Log("The KUBECONFIG environment variable is not as expected.")
		t.Logf("Expected: %s", testCase.ExpectKubeconfig)
		t.Logf("Actual  : %s", value)
		t.Fail()
		return "", true
	}

	for idx, actualElement := range searchPath[1:] {
		expectedElement := expectedPath[idx]
		if !strings.HasSuffix(actualElement, expectedElement) {
			t.Logf("The KUBECONFIG environment variable is not as expected.  Element %d is different.", idx)
			t.Logf("Expected suffix: %s", expectedElement)
			t.Logf("Actual         : %s", actualElement)
			t.Fail()
			return "", true
		}
	}

	// Return the first element, which is the local kubectl config file, so the caller can examine
	// it to make sure it's correct.
	return searchPath[0], false
}

func verifyTeleportProxyEnvVar(t *testing.T, output string, testCase *TestCase) bool {
	match := extractTeleportProxyEnvVar.FindStringSubmatch(output)
	if match == nil {
		if testCase.ExpectTeleportProxy == "" {
			return false
		}
		t.Log("Couldn't find the TELEPORT_PROXY environment variable in the output.")
		t.Logf("output: %s", output)
		t.Fail()
		return true
	}
	value := match[1]
	if value != testCase.ExpectTeleportProxy {
		t.Log("The TELEPORT_PROXY environment variable is not as expected.")
		t.Logf("Expected: %s", testCase.ExpectTeleportProxy)
		t.Logf("Actual  : %s", value)
		t.Fail()
		return true
	}
	return false
}

func verifyKubectlExe(t *testing.T, output string, testCase *TestCase) bool {
	match := extractKubectlExe.FindStringSubmatch(output)
	if match == nil {
		t.Log("Couldn't find the kubectl executable name in the output.")
		t.Logf("output: %s", output)
		t.Fail()
		return true
	}
	value := match[1]
	if value != testCase.ExpectKubectlExe {
		t.Log("The kubectl executable name is not as expected.")
		t.Logf("Expected: %s", testCase.ExpectKubectlExe)
		t.Logf("Actual  : %s", value)
		t.Fail()
		return true
	}
	return false
}

func verifyPrompt(t *testing.T, output string, testCase *TestCase) bool {
	match := extractPrompt.FindStringSubmatch(output)
	if match == nil {
		if testCase.ExpectPrompt == "" {
			// This is expected.
			return false
		}
		t.Log("Couldn't find the prompt info in the output.")
		t.Logf("output: %s", output)
		t.Fail()
		return true
	}
	value := match[1]
	if value != testCase.ExpectPrompt {
		t.Log("The kset prompt is not as expected.")
		t.Logf("Expected: %s", testCase.ExpectPrompt)
		t.Logf("Actual  : %s", value)
		t.Fail()
		return true
	}
	return false
}

func verifyLocalKubectlConfigFile(t *testing.T, localConfigFileName string, testCase *TestCase) bool {
	expectedLocalConfigFilePath := filepath.Join("testdata", "configs", fmt.Sprintf("local-config.%s.yaml", testCase.ExpectLocalConfigFile))
	expectedContents, err := readYamlFile(expectedLocalConfigFilePath)
	if err != nil {
		t.Errorf("Error examining expected local kubectl config file: %v", err)
		if errors.Is(err, os.ErrNotExist) {
			// Dump the actual file, to help the developer prime the "expected" file after the first run.
			t.Log("Actual local config file:")
			rawContents, err := os.ReadFile(localConfigFileName)
			if err != nil {
				t.Logf("... Error reading local config file \"%s\":  %v", localConfigFileName, err)
			} else {
				t.Log("\n" + string(rawContents))
			}
		}
		return true
	}

	actualContents, err := readYamlFile(localConfigFileName)
	if err != nil {
		t.Errorf("Error examining actual local kubectl config file: %v", err)
		return true
	}

	if !reflect.DeepEqual(expectedContents, actualContents) {
		t.Log("The local kubectl configuration file is not as expected.")
		t.Logf("Expected file \"%s\":", expectedLocalConfigFilePath)
		rawContents, err := os.ReadFile(expectedLocalConfigFilePath)
		if err != nil {
			t.Logf("... Error reading file:  %v", err)
		} else {
			t.Log(string(rawContents))
		}
		t.Logf("Actual file \"%s\":", localConfigFileName)
		rawContents, err = os.ReadFile(localConfigFileName)
		if err != nil {
			t.Logf("... Error reading file:  %v", err)
		} else {
			t.Log(string(rawContents))
		}
		t.Fail()
		return true
	}

	return false
}

func readYamlFile(path string) (map[string]interface{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	context := make(map[string]interface{})
	err = yaml.NewDecoder(file).Decode(context)
	if err != nil {
		return nil, fmt.Errorf("Error parsing file \"%s\": %v", path, err)
	}
	return context, nil
}

var emptyPreferences = config.KconfigPreferences{}

func copyConfigFile(t *testing.T, filename string, preferences *config.KconfigPreferences) error {
	target := filepath.Join(testHomeDir, ".kube", filename)
	err := os.Remove(target)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	sourcePath := filepath.Join("testdata", "configs", filename)
	//sourceFile, err := os.Open(sourcePath)
	//if err != nil {
	//	if err == os.ErrNotExist {
	//		return nil
	//	}
	//	return err
	//}
	//
	//defer sourceFile.Close()

	targetFile, err := os.Create(target)
	if err != nil {
		return err
	}

	defer targetFile.Close()

	if preferences != nil && *preferences != emptyPreferences {
		kconfig := config.Kconfig{}
		kconfig.Preferences = *preferences
		if kconfig.Preferences.BaseKubeconfig != "" {
			kconfig.Preferences.BaseKubeconfig = strings.ReplaceAll(kconfig.Preferences.BaseKubeconfig, "$HOME", testHomeDir)
		}
		err = yaml.NewEncoder(targetFile).Encode(kconfig)
		if err != nil {
			return fmt.Errorf("Error encoding preferences to file \"%s\": %v", target, err)
		}
	}

	// Read contents into memory so we can easy string substitutions
	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("Error reading from file \"%s\": %v", sourcePath, err)
	}

	contents = bytes.ReplaceAll(contents, []byte("$HOME"), []byte(testHomeDir))
	_, err = targetFile.Write(contents)
	if err != nil {
		return fmt.Errorf("Error writing to file \"%s\": %v", target, err)
	}

	//_, err = io.Copy(targetFile, sourceFile)
	//if err != nil {
	//	return fmt.Errorf("Error copying from file \"%s\" to \"%s\": %v", sourcePath, target, err)
	//}
	////t.Logf("Copied file \"%s\" to \"%s\"", sourcePath, target)

	t.Cleanup(func() {
		err := os.Remove(target)
		if err != nil && err != os.ErrNotExist {
			t.Logf("Error removing file \"%s\" after testcase: %v", target, err)
		}
		//t.Logf("Removed file \"%s\"", target)
	})

	return nil
}

var (
	trueVal = true
	truePtr = &trueVal

	falseVal = false
	falsePtr = &falseVal
)

// GetBoolPtr is a convenience function for returning a boolean pointer, useful
// for optional boolean fields.  The caller should NOT change the value of the
// boolean that is referenced by the pointer.
func GetBoolPtr(val bool) *bool {
	if val {
		return truePtr
	}
	return falsePtr
}
