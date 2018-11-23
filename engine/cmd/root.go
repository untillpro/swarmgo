/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cmd

import (
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	swarmgoConfigFileName      = "swarmgo-config.yml"
	traefikComposeFileName     = "traefik.yml"
	traefikTestComposeFileName = "traefik-http.yml"
	dockerComposeFileName      = "docker-compose.yml"
)

var clusterFile = map[string]interface{
"OrganizationName"   string `yaml:"OrganizationName"`
	ClusterName        string `yaml:"ClusterName"`
	ClusterUser        string `yaml:"ClusterUser"`
	Docker             string `yaml:"Docker"`
	Alertmanager       string `yaml:"Alertmanager"`
	NodeExporter       string `yaml:"NodeExporter"`
	Prometheus         string `yaml:"Prometheus"`
	Grafana            string `yaml:"Grafana"`
	Traefik            string `yaml:"Traefik"`
	Cadvisor           string `yaml:"Cadvisor"`
	ACMEEnabled        bool   `yaml:"ACMEEnabled"`
	Domain             string `yaml:"Domain"`
	Email              string `yaml:"Email"`
	GrafanaUser        string `yaml:"GrafanaUser"`
	GrafanaPassword    string `yaml:"GrafanaPassword"`
	PrometheusUser     string `yaml:"PrometheusUser"`
	PrometheusPassword string `yaml:"PrometheusPassword"`
	TraefikUser        string `yaml:"TraefikUser"`
	TraefikPassword    string `yaml:"TraefikPassword"`
	WebhookURL         string `yaml:"WebhookURL"`
	ChannelName        string `yaml:"ChannelName"`
	AlertmanagerUser   string `yaml:"AlertmanagerUser"`
	NodeID             string `yaml:"NodeID"`
}

var cfgFile string
var logs bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "swarmgo",
	Short: "SwarmGo is application to create docker cluster in swarm mode",
	Long: `SwarmGo is application to create and configure docker cluster in swarm mode and install necessary tools
to work with him`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	//	Run: func(cmd *cobra.Command, args []string) { },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&logs, "logs", "l", false, "Redirect logs to ./logs/log.log")
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".swarmgo" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".swarmgo")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
