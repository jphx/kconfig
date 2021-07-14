package common

import "go.uber.org/zap"

// CommonOptions describes the command-line options for the program that are common to all
// subcommands.
var CommonOptions struct {
	Debug bool `long:"debug" description:"Enable debug-level messages"`
}

var RootLogger = initializeLogger()
var LoggingLevel zap.AtomicLevel

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
