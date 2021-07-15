package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/jphx/kconfig/config"
)

func main() {
	me, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to deduce location of this executable: %v", err)
		os.Exit(1)
	}
	//fmt.Fprintf(os.Stderr, "my absolute path is: %s\n", me)

	argsToPassToKubectl := os.Args[1:]
	argsToPassToKubectl, kubectlExecutable := maybeCreateLocalConfigFile(argsToPassToKubectl)

	if kubectlExecutable == "" {
		kubectlExecutable = os.Getenv("_KCONFIG_KUBECTL")
		if kubectlExecutable == "" {
			kubectlExecutable = "kubectl"
		}
	}

	fmt.Fprintf(os.Stderr, "Looking up executable: %s\n", kubectlExecutable)
	executable, err := findExecutable(kubectlExecutable, me)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Found executable at: %s\n", executable)
	kev := os.Getenv("KUBECONFIG")
	if kev != "" {
		fmt.Fprintf(os.Stderr, "KUBECONFIG env var is: %s\n", kev)
	}

	var argv []string
	argv = append(argv, executable)
	argv = append(argv, argsToPassToKubectl...)
	err = unix.Exec(executable, argv, os.Environ())
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}

func maybeCreateLocalConfigFile(argsToPassToKubectl []string) ([]string, string) {
	if len(argsToPassToKubectl) < 2 {
		return argsToPassToKubectl, ""
	}

	firstArg := argsToPassToKubectl[0]
	if firstArg != "--kconfig" && firstArg != "-k" {
		return argsToPassToKubectl, ""
	}

	nickname := argsToPassToKubectl[1]
	if strings.HasPrefix(nickname, "-") {
		fmt.Fprintf(os.Stderr, "The kconfig nickname is missing after the \"%s\" option.", firstArg)
		os.Exit(1)
	}

	argsToPassToKubectl = argsToPassToKubectl[2:]

	kubeconfig, kubectlExecutable, _ := config.CreateLocalKubectlConfigFile(nickname, nil, false)

	// Set the KUBECONFIG environment variable, which will be in the environment passed to the
	// kubectl executable.  This will cause it to use this local kubectl configuration file.
	err := os.Setenv("KUBECONFIG", kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "The kconfig nickname is missing after the \"%s\" option.", firstArg)
		os.Exit(1)
	}

	return argsToPassToKubectl, kubectlExecutable
}

func findExecutable(name string, skip string) (string, error) {
	slash := strings.IndexByte(name, '/')
	if slash != -1 {
		if isExecutable(name) {
			if isSameFile(name, skip) {
				return "", fmt.Errorf("Specified path name is this executable: %s", skip)
			}
			return name, nil
		}
		return "", fmt.Errorf("Executable not found (or is not executable): %s", name)
	}

	path := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		path := filepath.Join(dir, name)
		if isSameFile(path, skip) {
			// Skip me
			continue
		}

		if isExecutable(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("Executable not found or is not executable: %s", name)
}

func isExecutable(file string) bool {
	fileInfo, err := os.Stat(file)
	if err != nil {
		return false
	}
	fileMode := fileInfo.Mode()
	if !fileMode.IsDir() && fileMode&0111 != 0 {
		return true
	}
	return false
}

func isSameFile(path string, skip string) bool {
	//fmt.Fprintf(os.Stderr, "Checking \"%s\" against \"%s\".\n", path, skip)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	//fmt.Fprintf(os.Stderr, "Absolute path is: %s\n", absPath)
	//fmt.Fprintf(os.Stderr, "same is: %v\n", absPath == skip)

	return absPath == skip
}
