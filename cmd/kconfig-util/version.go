package main

import (
	"fmt"

	"github.com/jphx/kconfig/common"
)

type versionCommandOptions struct {
}

var versionOptions versionCommandOptions

func (o *versionCommandOptions) Usage() string {
	return ""
}

func (o *versionCommandOptions) Execute(args []string) error {
	commandProcessor = versionProcessor
	commandName = "version"

	if len(args) > 0 {
		return fmt.Errorf("Unrecognized positional arguments provided.")
	}

	return nil
}

func versionProcessor(positionalArgs []string) {
	fmt.Println(common.Version)
}

func init() {
	_, err := parser.AddCommand("version",
		"Print the kconfig version",
		"Print the kconfig version to standard output.",
		&versionOptions)

	if err != nil {
		panic(fmt.Sprintf("Error adding command for parsing: %v", err))
	}
}
