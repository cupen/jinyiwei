package logs

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Logger(level string) *zap.Logger {
	lv := zap.InfoLevel
	if level == "debug" {
		lv = zap.DebugLevel
	}
	conf := zap.Config{
		Level:       zap.NewAtomicLevelAt(lv),
		Development: false,
		Sampling:    nil,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.RFC3339TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	log, err := conf.Build()
	if err != nil {
		panic(err)
	}
	return log
}
