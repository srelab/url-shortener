package logger

import (
	"io"
	"os"

	"fmt"
	"path"
	"strings"

	"github.com/natefinch/lumberjack"
	"github.com/srelab/common/log"
	"github.com/srelab/url-shortener/pkg/g"
)

var (
	logger *log.Logger
)

const (
	LevelDebug = iota + 1
	LevelInfo
	LevelError
	LevelWarning
)

func updateLevel(logLevel string) {
	switch strings.ToLower(logLevel) {
	case "debug":
		logger.SetLevel(LevelDebug)
	case "info":
		logger.SetLevel(LevelInfo)
	case "warn":
		logger.SetLevel(LevelWarning)
	case "error":
		logger.SetLevel(LevelError)
	default:
		logger.SetLevel(LevelInfo)
	}
}

func Error(v ...interface{}) {
	logger.Error(v...)
}

func Errorf(format string, v ...interface{}) {
	logger.Errorf(format, v...)
}

func Warn(v ...interface{}) {
	logger.Warn(v...)
}

func Warnf(format string, v ...interface{}) {
	logger.Warnf(format, v...)
}

func Info(v ...interface{}) {
	logger.Info(v...)
}

func Infof(format string, v ...interface{}) {
	logger.Infof(format, v...)
}

func Debug(v ...interface{}) {
	logger.Debug(v...)
}

func Debugf(format string, v ...interface{}) {
	logger.Debugf(format, v...)
}

func Fatal(v ...interface{}) {
	logger.Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	logger.Fatalf(format, v...)
}

func InitLogger() {
	logger = log.New(g.NAME)
	logger.SetLevel(LevelInfo)
	logger.SetHeader("[${level}][${prefix}][${time_rfc3339][${short_file}#]${line}: ")
	logger.SetOutput(GetLogWriter(fmt.Sprintf("%s.log", g.NAME)))
}

func GetLogWriter(filename string) io.Writer {
	if g.GetConfig().Log.Level == "debug" {
		SetLogLevel(g.GetConfig().Log.Level)
		return io.MultiWriter(os.Stdout, &lumberjack.Logger{
			Filename:   path.Join(g.GetConfig().Log.Dir, filename),
			MaxSize:    500,
			MaxBackups: 3,
			MaxAge:     28,
		})
	}

	return &lumberjack.Logger{
		Filename:   path.Join(g.GetConfig().Log.Dir, filename),
		MaxSize:    500,
		MaxBackups: 3,
		MaxAge:     28,
	}
}

func SetLogLevel(logLevel string) string {
	if len(logLevel) == 0 {
		logLevel = "info"
	}

	updateLevel(logLevel)
	return logLevel
}

func GetLogLevel() int {
	switch strings.ToLower(g.GetConfig().Log.Level) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarning
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}
