/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/karrick/tparse/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var promUriString string
var promUri *url.URL
var timeStartString string
var timeEndString string
var timeStepString string
var timeStart time.Time
var timeEnd time.Time
var timeStep time.Duration
var outputFormat string
var showAllApps bool
var showDebug bool
var suppressWarnings bool

const (
	OUTPUT_INTERACTIVE = "interactive"
	OUTPUT_TABLE       = "table"
	OUTPUT_DETAIL      = "detail"
	OUTPUT_YAML        = "yaml"
	OUTPUT_SERVO       = "servo.yaml"
)

// constant table - format types, keep in sync with OUTPUT_xxx constants above
func getOutputFormats() []string {
	return []string{OUTPUT_INTERACTIVE, OUTPUT_TABLE, OUTPUT_DETAIL, OUTPUT_YAML, OUTPUT_SERVO}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "opsani-ignite [<namespace> [<deployment>]]",
	Short: "Opsani Ignite for Kubernetes",
	Long: `Opsani Ignite looks through the performance history of 
application workloads running on Kubernetes and identifies optimization opportunities.

For each application it finds, it evaluates what can be optimized and displays
a list of optimization candidates in preferred order of onboarding.`,
	PersistentPreRunE: validateFlags,
	Args:              cobra.MaximumNArgs(2),
	Run:               runIgnite,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().SortFlags = false // also requires Flags().SortFlag = false
	rootCmd.Flags().SortFlags = false           // also requires PersistentFlags().SortFlag = false

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.opsani-ignite.yaml)")

	rootCmd.PersistentFlags().StringVarP(&promUriString, "prometheus-url", "p", "", "URI to Prometheus API (typically port-forwarded to localhost using kubectl)")
	rootCmd.MarkPersistentFlagRequired("prometheus-url") // TODO: this doesn't seem to do anything, enforcing explicitly in parser function
	viper.BindPFlag("prometheus-url", rootCmd.PersistentFlags().Lookup("prometheus-url"))

	rootCmd.PersistentFlags().StringVar(&timeStartString, "start", "-7d", "Analysis start time, in RFC3339 or relative form")
	rootCmd.PersistentFlags().StringVar(&timeEndString, "end", "-0d", "Analysis end time, in RFC3339 or relative form")
	rootCmd.PersistentFlags().StringVar(&timeStepString, "step", "1d", "Time resolution, in relative form")

	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", fmt.Sprintf("Output format (%v)", strings.Join(getOutputFormats(), "|")))
	rootCmd.PersistentFlags().BoolVarP(&showAllApps, "show-all", "a", false, "Show all apps, including unoptimizable")
	rootCmd.PersistentFlags().BoolVar(&showDebug, "debug", false, "Display tracing/debug information to stderr")
	rootCmd.PersistentFlags().BoolVarP(&suppressWarnings, "quiet", "q", false, "Suppress warning and info level messages")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".opsani-ignite" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".opsani-ignite")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func parseRequiredUriFlag(uri **url.URL, text string, flag string) error {
	if text == "" {
		return fmt.Errorf("Required parameter %q not specified", flag)
	}
	var err error
	*uri, err = url.ParseRequestURI(text)
	if err != nil {
		return fmt.Errorf("Invalid URL for parameter %q: %v", flag, err)
	}
	return nil
}

func parseInstant(s string, option string) (instant time.Time, err error) {
	now := time.Now()
	if strings.HasPrefix(s, "-") {
		instant, err = tparse.AddDuration(now, s)
		if err != nil {
			err = fmt.Errorf("error parsing %v (relative): %v", option, err)
		}
	} else {
		instant, err = tparse.Parse(time.RFC3339, s)
		if err != nil {
			err = fmt.Errorf("error parsing %v (absolute): %v", option, err)
		}
	}
	return
}

func validateFlags(cmd *cobra.Command, args []string) error {
	var err error

	// check flag dependencies
	if suppressWarnings && showDebug {
		return fmt.Errorf("--quiet and --debug flags cannot be combined")
	}

	// check output format
	if outputFormat == "" {
		// smart select: detail view if a single app is specified; table view for multiple apps
		if len(args) >= 2 { // namespace + deployment specifies a single app
			outputFormat = OUTPUT_DETAIL
		} else {
			outputFormat = OUTPUT_TABLE
		}
	} else {
		outputFormatValid := false
		for _, f := range getOutputFormats() {
			if outputFormat == f {
				outputFormatValid = true
				break
			}
		}
		if !outputFormatValid {
			return fmt.Errorf("--output format must be one of %v", getOutputFormats())
		}
	}

	// -- Time intervals parse and check
	timeStart, err = parseInstant(timeStartString, "--start")
	if err != nil {
		return err
	}
	timeEnd, err = parseInstant(timeEndString, "--end")
	if err != nil {
		return err
	}
	timeStep, err = tparse.AbsoluteDuration(timeStart, timeStepString)
	if err != nil {
		return fmt.Errorf("Could not parse time resolution: %v", err)
	}
	if !timeStart.Before(timeEnd) {
		return fmt.Errorf("Analysis start time must be earlier than end time")
	}
	if timeStep < time.Minute {
		return fmt.Errorf("Analysis time resolution must be at least 1 minute (found %v)", timeStep)
	} else if timeStep > 24*time.Hour {
		return fmt.Errorf("Analysis time resolution must be at shorter than a day (found %v)", timeStep)
	}
	if timeEnd.Sub(timeStart)/timeStep < 2 {
		return fmt.Errorf("Analysis time & resolution should allow for at least 2 samples")
	}

	// check prometheus URI
	return parseRequiredUriFlag(&promUri, promUriString, "-p/--prometheus-url")
}
