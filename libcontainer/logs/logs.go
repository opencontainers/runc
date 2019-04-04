package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

// loggingConfigured will be set once logging has been configured via invoking `ConfigureLogging`.
// Subsequent invocations of `ConfigureLogging` would be no-op
var loggingConfigured = false

type LoggingConfiguration struct {
	IsDebug     bool
	LogFormat   string
	LogFilePath string
	LogPipeFd   string
}

func ForwardLogs(logPipe io.Reader) {
	type jsonLog struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	lineReader := bufio.NewReader(logPipe)
	for {
		line, err := lineReader.ReadBytes('\n')
		if len(line) > 0 {
			processEntry(line)
		}
		if err == io.EOF {
			logrus.Debugf("log pipe has been closed: %+v", err)
			return
		}
		if err != nil {
			logrus.Errorf("log pipe read error: %+v", err)
		}
	}
}

func processEntry(text []byte) {
	type jsonLog struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	var jl jsonLog
	if err := json.Unmarshal(text, &jl); err != nil {
		logrus.Errorf("failed to decode %q to json: %+v", text, err)
		return
	}

	lvl, err := logrus.ParseLevel(jl.Level)
	if err != nil {
		logrus.Errorf("failed to parse log level %q: %v\n", jl.Level, err)
		return
	}
	log(lvl, jl.Msg)
}

func log(level logrus.Level, args ...interface{}) {
	switch level {
	case logrus.PanicLevel:
		logrus.Panic(args...)
	case logrus.FatalLevel:
		logrus.Fatal(args...)
	case logrus.ErrorLevel:
		logrus.Error(args...)
	case logrus.WarnLevel:
		logrus.Warn(args...)
	case logrus.InfoLevel:
		logrus.Info(args...)
	case logrus.DebugLevel:
		logrus.Debug(args...)
	default:
		logrus.Warnf("Unsupported log level %v while trying to log '%#v'", level, args)
	}
}

func ConfigureLogging(loggingConfig *LoggingConfiguration) error {
	if loggingConfigured {
		logrus.Debug("logging has been already configured")
		return nil
	}

	configureLogLevel(loggingConfig)
	if err := configureLogOutput(loggingConfig); err != nil {
		return err
	}
	if err := configureLogFormat(loggingConfig); err != nil {
		return err
	}

	loggingConfigured = true
	return nil
}

func configureLogLevel(loggingConfig *LoggingConfiguration) {
	if loggingConfig.IsDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func configureLogOutput(loggingConfig *LoggingConfiguration) error {
	if loggingConfig.LogFilePath != "" {
		return configureLogFileOutput(loggingConfig.LogFilePath)
	}

	if loggingConfig.LogPipeFd != "" {
		logPipeFdInt, err := strconv.Atoi(loggingConfig.LogPipeFd)
		if err != nil {
			return fmt.Errorf("failed to convert _LIBCONTAINER_LOGPIPE environment variable value %q to int: %v", loggingConfig.LogPipeFd, err)
		}
		configureLogPipeOutput(logPipeFdInt)
		return nil
	}

	return nil
}

func configureLogPipeOutput(logPipeFd int) {
	logrus.SetOutput(os.NewFile(uintptr(logPipeFd), "logpipe"))
}

func configureLogFileOutput(logFilePath string) error {
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0644)
	if err != nil {
		return err
	}
	logrus.SetOutput(f)
	return nil
}

func configureLogFormat(loggingConfig *LoggingConfiguration) error {
	switch loggingConfig.LogFormat {
	case "text":
		// retain logrus's default.
	case "json":
		logrus.SetFormatter(new(logrus.JSONFormatter))
	default:
		return fmt.Errorf("unknown log-format %q", loggingConfig.LogFormat)
	}
	return nil
}
