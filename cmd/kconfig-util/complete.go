package main

import (
	"fmt"
	"strings"

	"github.com/jphx/kconfig/config"
)

type completeCommandOptions struct {
}

var completeOptions completeCommandOptions

func (o *completeCommandOptions) Usage() string {
	return "nickname-prefix"
}

func (o *completeCommandOptions) Execute(args []string) error {
	commandProcessor = completeProcessor
	commandName = "complete"

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

func completeProcessor(positionalArgs []string) {
	nicknamePrefix := positionalArgs[0]

	kconfig := config.GetKconfig()
	for nickname := range kconfig.Nicknames {
		if strings.HasPrefix(nickname, nicknamePrefix) {
			fmt.Println(nickname)
		}
	}
}

func init() {
	_, err := parser.AddCommand("complete",
		"Print eligible auto-completion results",
		"To be used for shell autocompletion.  It prints the list of nicknames that are valid "+
			"completions for the part that has been entered so far",
		&completeOptions)

	if err != nil {
		panic(fmt.Sprintf("Error adding command for parsing: %v", err))
	}
}
