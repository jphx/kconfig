package common

import "go.uber.org/zap"

// Version contains the kconfig version.  It's used by the "version" subcommand.  It's intended to
// be set as a build-time option using the "ldflags" option to "go build".  E.g.,
//
// go build -o bin/ -ldflags="-X github.com/jphx/kconfig/common.Version=${VERSION}" ./...
var Version string

// CommonOptions describes the command-line options for the program that are common to all
// subcommands.
var CommonOptions struct {
	Debug bool `long:"debug" description:"Enable debug-level messages"`
}

// RootLogger is the root logger for the application.
var RootLogger = initializeLogger()

// LoggingLevel controls the current logging level.  It's initially set to Info.
var LoggingLevel zap.AtomicLevel

// CreateLogger creates a named child logger of the root logger.
func CreateLogger(name string) *zap.SugaredLogger {
	return RootLogger.Named(name)
}

func initializeLogger() *zap.SugaredLogger {
	zapConfig := zap.NewProductionConfig()
	LoggingLevel = zapConfig.Level
	//if debug {
	//	loggingLevel.SetLevel(zap.DebugLevel)
	//}

	zapConfig.Encoding = "console"
	//zapConfig.Development = debug
	zapConfig.DisableCaller = true
	//zapConfig.DisableStacktrace = true

	zapLogger, err := zapConfig.Build()
	if err != nil {
		panic("Unable to set up logger")
	}
	return zapLogger.Sugar()
}
