package config

import (
	"fmt"
	"os"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func ReadKubeConfig() *clientcmdapi.Config {
	configAccess := clientcmd.NewDefaultPathOptions()
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading kubectl config file(s): %v\n", err)
		os.Exit(1)
	}

	//fmt.Printf("There are %d contexts\n", len(config.Contexts))
	////for name, _ := range config.Contexts {
	////	fmt.Println(name)
	////}
	//fmt.Printf("The active context is \"%s\"\n", config.CurrentContext)

	return config
}
