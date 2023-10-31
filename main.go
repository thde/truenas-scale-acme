package main

import (
	"context"
	"os"
	"runtime/debug"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/thde/truenas-scale-acme/internal/cli"
)

var (
	version   string
	commit    string
	date      string
	goVersion string
)

func main() {
	enc := zap.NewProductionEncoderConfig()
	enc.EncodeTime = zapcore.RFC3339TimeEncoder
	enc.EncodeLevel = zapcore.CapitalColorLevelEncoder
	enc.ConsoleSeparator = " "

	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(enc),
		os.Stderr,
		zap.InfoLevel,
	))

	if err := cli.Run(context.Background(), logger, &cli.BuildInfo{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: goVersion,
	}); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if version == "" {
		version = info.Main.Version
	}

	goVersion = info.GoVersion

	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			commit = kv.Value
		case "vcs.time":
			date = kv.Value
		}
	}
}
