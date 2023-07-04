package common

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
)

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

func getLogWriter(filename string, stdOut bool) zapcore.WriteSyncer {
	var ws io.Writer
	if filename != "" {
		lumberJackLogger := &lumberjack.Logger{
			Filename:   filename,
			MaxSize:    1,
			MaxBackups: 30,
			MaxAge:     7,
			LocalTime:  true,
			Compress:   false,
		}
		if stdOut {
			ws = io.MultiWriter(lumberJackLogger, os.Stdout)
		} else {
			ws = io.MultiWriter(lumberJackLogger, os.Stderr)
		}
	} else {
		if stdOut {
			ws = io.MultiWriter(os.Stdout)
		} else {
			ws = io.MultiWriter(os.Stderr)
		}
	}
	return zapcore.AddSync(ws)
}
