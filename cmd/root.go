/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/spf13/viper"
)

var cfgFile string
var promUriString string
var promUri *url.URL
var namespace string
var deployment string
var outputFormat string
var showAllApps bool
var showDebug bool

const (
	OUTPUT_TABLE  = "table"
	OUTPUT_DETAIL = "detail"
	OUTPUT_YAML   = "yaml"
)

// constant table - format types, keep in sync with OUTPUT_xxx constants above
func getOutputFormats() []string {
	return []string{OUTPUT_TABLE, OUTPUT_DETAIL, OUTPUT_YAML}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "opsani-ignite",
	Short: "Opsani Ignite for Kubernetes",
	Long: `Opsani Ignite looks through the performance history of 
application workloads running on Kubernetes and identifies optimization opportunities.

For each application it finds, it evaluates what can be optimized and displays
a list of optimization candidates in preferred order of onboarding.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// check dependent flags
		if deployment != "" && namespace == "" {
			return fmt.Errorf("--deployment flag requires --namespace flag")
		}

		// check output format
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

		// check prometheus URI
		return parseRequiredUriFlag(&promUri, promUriString, "-p/--prometheus-url")
	},
	Run: runIgnite,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().SortFlags = false //TODO doesn't work

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.opsani-ignite.yaml)")

	rootCmd.PersistentFlags().StringVarP(&promUriString, "prometheus-url", "p", "", "URI to Prometheus API (typically port-forwarded to localhost using kubectl)")
	rootCmd.MarkPersistentFlagRequired("prometheus-url") // TODO: this doesn't seem to do anything, enforcing explicitly in parser function
	viper.BindPFlag("prometheus-url", rootCmd.PersistentFlags().Lookup("prometheus-url"))

	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Limit search to a namespace")
	rootCmd.PersistentFlags().StringVarP(&deployment, "deployment", "d", "", "Limit search to a deployment name in namespace")

	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", OUTPUT_TABLE, "Output format")
	rootCmd.PersistentFlags().BoolVarP(&showAllApps, "show-all", "a", false, "Show all apps, including unoptimizable")
	rootCmd.PersistentFlags().BoolVar(&showDebug, "debug", false, "Display tracing/debug information to stderr")
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
