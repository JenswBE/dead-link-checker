package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/adhocore/gronx/pkg/tasker"
	flag "github.com/spf13/pflag"

	"github.com/JenswBE/dead-link-checker/cmd/config"
	"github.com/JenswBE/dead-link-checker/internal"
)

func main() {
	// Parse flags
	verbose := flag.BoolP("verbose", "v", false,
		"Enables verbose output. Will be enabled if either this flag, config option or env var is provided.")
	configPath := flag.StringP("config", "c", "./config.yml", "Path to the config file")
	printJSON := flag.Bool("json", false, "Print all site reports as JSON to stdout")
	runNow := flag.Bool("now", false, "Overrides cron and forces an immediate check")
	flag.Parse()

	// Setup logging
	logLevel := &slog.LevelVar{}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	// Parse config
	delicConfig, err := config.ParseConfig(*configPath)
	if err != nil {
		slogFatal("Failed to parse config", "error", err)
	}

	// Setup Debug logging if enabled
	if delicConfig.Verbose || *verbose {
		logLevel.Set(slog.LevelDebug)
		slog.Debug("Debug logging enabled")
	}

	// Create manager
	manager := internal.NewManager()

	// Run DeLiC
	if delicConfig.Cron == "" || *runNow {
		// Run once
		if err = runDeLiC(context.Background(), manager, delicConfig, *printJSON); err != nil {
			slogFatal("Error while running DeLiC", "error", err)
		}
	} else {
		// Run at cron interval
		slog.Info("DeLiC started with cron", "spec", delicConfig.Cron)
		tasker.
			New(tasker.Option{Verbose: delicConfig.Verbose}).
			Task(delicConfig.Cron, newDeLiCTask(manager, delicConfig, *printJSON)).
			Run()
	}
}

func runDeLiC(ctx context.Context, manager *internal.Manager, delicConfig *config.Config, printJSON bool) error {
	// Run manager
	reports := manager.Run(ctx, delicConfig)

	// Print JSON results if enabled
	if printJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "    ")
		if err := encoder.Encode(reports); err != nil {
			// Both log and return error to have correct severity in logs
			slog.Error("Failed to print reports as JSON to stdout", "error", err)
			return fmt.Errorf("failed to print reports as JSON to stdout: %w", err)
		}
	}
	return nil
}

func slogFatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}

func newDeLiCTask(manager *internal.Manager, delicConfig *config.Config, printJSON bool) tasker.TaskFunc {
	return func(ctx context.Context) (int, error) {
		if err := runDeLiC(ctx, manager, delicConfig, printJSON); err != nil {
			return 1, err
		}
		return 0, nil
	}
}
