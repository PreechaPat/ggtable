package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var zapLog *zap.Logger

func InitLogger(level zapcore.Level) error {

	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(level) // Set to desired level

	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("Jan _2 15:04:05.000000000")
	encoderConfig.StacktraceKey = "" // to hide stacktrace info
	config.EncoderConfig = encoderConfig

	var err error
	zapLog, err = config.Build(zap.AddCallerSkip(1))
	if err != nil {
		return err
	}
	return nil
}

func Info(message string, fields ...zap.Field) {
	zapLog.Info(message, fields...)
}

func Warn(message string, fields ...zap.Field) {
	zapLog.Warn(message, fields...)
}

func Debug(message string, fields ...zap.Field) {
	zapLog.Debug(message, fields...)
}

func Error(message string, fields ...zap.Field) {
	zapLog.Error(message, fields...)
}

func Fatal(message string, fields ...zap.Field) {
	zapLog.Fatal(message, fields...)
}

// Sync flushes any buffered log entries
func Sync() error {
	return zapLog.Sync()
}
