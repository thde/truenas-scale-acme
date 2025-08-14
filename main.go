package main

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/thde/truenas-scale-acme/internal/cli"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	version   string
	commit    string
	date      string
	goVersion string
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := initLogger(os.Stderr)

	if err := cli.Run(ctx, logger, &cli.BuildInfo{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: goVersion,
	}); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func initLogger(out *os.File) *zap.Logger {
	enc := zap.NewProductionEncoderConfig()
	enc.EncodeTime = zapcore.RFC3339TimeEncoder
	enc.ConsoleSeparator = " "
	enc.EncodeLevel = zapcore.CapitalLevelEncoder
	if isatty.IsTerminal(out.Fd()) {
		enc.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(enc),
		out,
		zap.InfoLevel,
	))
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
