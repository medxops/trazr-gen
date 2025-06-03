// Copyright The OpenTelemetry Authors
// Copyright (c) 2018 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/trazr-gen/internal/trazr-gen"

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/medxops/trazr-gen/internal/common"
	"github.com/medxops/trazr-gen/pkg/logs"
	"github.com/medxops/trazr-gen/pkg/metrics"
	"github.com/medxops/trazr-gen/pkg/traces"
)

var (
	tracesCfg  *traces.Config
	metricsCfg *metrics.Config
	logsCfg    *logs.Config
	configFile string
)

const rootHelpTemplate = `
╔══════════════════════════════════════════════════════════╗
║                    T R A Z R - G E N                     ║
║              OpenTelemetry Signal Generator              ║
╚══════════════════════════════════════════════════════════╝


{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}{{end}}
{{if .Runnable}}
Usage:
  {{.UseLine}}
{{end}}
{{if .HasAvailableSubCommands}}
Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}
{{end}}
{{if .HasExample}}
Examples:
{{.Example}}
{{end}}
{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}
{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}
{{if .HasHelpSubCommands}}
Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}
{{end}}{{end}}
{{end}}
Tip: Run 'trazr-gen [command] --help' for more details on each subcommand.`

// rootCmd is the root command on which will be run children commands
var rootCmd = &cobra.Command{
	Use:     "trazr-gen",
	Short:   "Trazr-gen simulates a client generating traces, metrics, and logs",
	Example: "trazr-gen traces\ntrazr-gen metrics\ntrazr-gen logs",
}

// tracesCmd is the command responsible for sending traces
var tracesCmd = &cobra.Command{
	Use:     "traces",
	Short:   "Simulates a client generating traces. (Stability level: alpha)",
	Example: "trazr-gen traces",
	RunE: func(_ *cobra.Command, _ []string) error {
		logger, err := common.CreateLogger(tracesCfg.LogLevel, tracesCfg.TerminalOutput)
		if err != nil {
			return err
		}
		return traces.Start(tracesCfg, logger)
	},
}

// metricsCmd is the command responsible for sending metrics
var metricsCmd = &cobra.Command{
	Use:     "metrics",
	Short:   "Simulates a client generating metrics. (Stability level: development)",
	Example: "trazr-gen metrics",
	RunE: func(_ *cobra.Command, _ []string) error {
		logger, err := common.CreateLogger(metricsCfg.LogLevel, metricsCfg.TerminalOutput)
		if err != nil {
			return err
		}
		return metrics.Start(metricsCfg, logger)
	},
}

// logsCmd is the command responsible for sending logs
var logsCmd = &cobra.Command{
	Use:     "logs",
	Short:   "Simulates a client generating metrics. (Stability level: development)",
	Example: "trazr-gen logs",
	RunE: func(_ *cobra.Command, _ []string) error {
		logger, err := common.CreateLogger(logsCfg.LogLevel, logsCfg.TerminalOutput)
		if err != nil {
			return err
		}
		return logs.Start(logsCfg, logger)
	},
}

func init() {
	rootCmd.AddCommand(tracesCmd, metricsCmd, logsCmd)

	tracesCfg = traces.NewConfig()
	tracesCfg.Flags(tracesCmd.Flags())

	metricsCfg = metrics.NewConfig()
	metricsCfg.Flags(metricsCmd.Flags())

	logsCfg = logs.NewConfig()
	logsCfg.Flags(logsCmd.Flags())

	// Set custom help templates for each subcommand
	traces.SetHelpTemplateForCmd(tracesCmd)
	metrics.SetHelpTemplateForCmd(metricsCmd)
	logs.SetHelpTemplateForCmd(logsCmd)

	// Disabling completion command for end user
	// https://github.com/spf13/cobra/blob/master/shell_completions.md
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "use a config file instead of command line parameters (YAML)")
	if err := viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config")); err != nil {
		if logsCfg.TerminalOutput {
			fmt.Println("failed to bind config flag:", err)
		}
	}

	// Register log-level flag
	rootCmd.PersistentFlags().StringVar(&logsCfg.LogLevel, "log-level", logsCfg.LogLevel, "Log level: debug, info, warn, error")
	if err := viper.BindPFlag("log-level", rootCmd.PersistentFlags().Lookup("log-level")); err != nil {
		if logsCfg.TerminalOutput {
			fmt.Println("failed to bind log-level flag:", err)
		}
	}

	// Ensure config is loaded after flags are parsed
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		initConfig()

		if logsCfg.TerminalOutput {
			fmt.Println("╔══════════════════════════════════════════════════════════╗")
			fmt.Println("║                    T R A Z R - G E N                     ║")
			fmt.Println("║              OpenTelemetry Signal Generator              ║")
			fmt.Println("╚══════════════════════════════════════════════════════════╝")

			switch cmd.Name() {
			case "traces":
				common.ShowNonDefaultConfig(tracesCfg)
			case "metrics":
				common.ShowNonDefaultConfig(metricsCfg)
			case "logs":
				common.ShowNonDefaultConfig(logsCfg)
			}
		}
		return nil
	}

	rootCmd.SetHelpTemplate(rootHelpTemplate)
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
		viper.AutomaticEnv()
		if err := viper.ReadInConfig(); err != nil {
			fmt.Println("Error reading config file:", err)
			os.Exit(1)
		}
		// Unmarshal global/common fields into each config struct
		_ = viper.Unmarshal(tracesCfg)
		_ = viper.Unmarshal(metricsCfg)
		_ = viper.Unmarshal(logsCfg)
		// Unmarshal subcommand-specific fields if present
		if sub := viper.Sub("traces"); sub != nil {
			_ = sub.Unmarshal(tracesCfg)
		}
		if sub := viper.Sub("metrics"); sub != nil {
			_ = sub.Unmarshal(metricsCfg)
		}
		if sub := viper.Sub("logs"); sub != nil {
			_ = sub.Unmarshal(logsCfg)
		}
	} else {
		// No config file specified, just use environment variables and flags
		viper.AutomaticEnv()
	}
}

// Execute tries to run the input command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Error executing command:", err)
		os.Exit(1)
	}
}
