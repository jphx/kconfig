package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"go.uber.org/zap"

	"github.com/jphx/kconfig/common"
)

// parser is the command-line parser.  It is modified by init() functions of other files to add
// subcommands and their options.  We omit flags.PrintErrors from the options since this would
// cause help output to go to stdout, whereas we want it to go to stderr, lest it be confused by
// the caller as shell commands to issue.
var parser = flags.NewParser(&common.CommonOptions, flags.HelpFlag|flags.PassDoubleDash) //
var commandProcessor func(positionalArgs []string)
var commandName string

func main() {
	positionalArgs := parseOptions()
	if common.CommonOptions.Debug {
		common.LoggingLevel.SetLevel(zap.DebugLevel)
	}
	defer func() { _ = common.RootLogger.Sync() }()

	common.RootLogger.Debugf("Invoking command: %s", commandName)
	commandProcessor(positionalArgs)
}

// parseOptions parses the command-line options, returning only if they can be successfully parsed.
func parseOptions() []string {
	argsToParse := os.Args[1:]

	// Special case handling for the "kset -" subcommand, where we fetch the env var that describes
	// the previous environment and parse that instead.
	// Beware this test fails when the --debug option is specified.
	if len(argsToParse) == 2 && argsToParse[0] == "kset" && argsToParse[1] == "-" {
		previousKset := os.Getenv("_KCONFIG_OLDKSET")
		if previousKset == "" {
			fmt.Fprintln(os.Stderr, "A kconfig nickname of \"-\" can only be used when a kconfig environment was previously in effect.")
			os.Exit(1)
		}

		argsToParse = []string{"kset"}
		argsToParse = append(argsToParse, getArgsFromKsetArgs(previousKset)...)
	}

	positionalArgs, err := parser.ParseArgs(argsToParse)
	if err != nil {
		// Print errors, and even help output, to stderr.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Subcommands generally define an Execute() method that will check if positional arguments are
	// allowed.

	return positionalArgs
}
