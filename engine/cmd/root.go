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

type clusterFile struct {
	OrganizationName string `yaml:"OrganizationName"`
	ClusterName      string `yaml:"ClusterName"`
	RootUserName     string `yaml:"RootUserName"`
	ClusterUserName  string `yaml:"ClusterUserName"`
	Docker           string `yaml:"Docker"`
	Alertmanager     string `yaml:"Alertmanager"`
	NodeExporter     string `yaml:"NodeExporter"`
	Prometheus       string `yaml:"Prometheus"`
	Grafana          string `yaml:"Grafana"`
	Traefik          string `yaml:"Traefik"`
	Cadvisor         string `yaml:"Cadvisor"`
	Consul           string `yaml:"Consul"`
	ACMEEnabled      bool   `yaml:"ACMEEnabled"`
	Domain           string `yaml:"Domain"`
	Email            string `yaml:"Email"`
	GrafanaUser      string `yaml:"GrafanaUser"`
	GrafanaPassword    string `yaml:"GrafanaPassword"`
	PrometheusUser     string `yaml:"PrometheusUser"`
	PrometheusPassword string `yaml:"PrometheusPassword"`
	TraefikUser        string `yaml:"TraefikUser"`
	TraefikPassword    string `yaml:"TraefikPassword"`
	WebhookURL         string `yaml:"WebhookURL"`
	ChannelName        string `yaml:"ChannelName"`
	AlertmanagerUser   string `yaml:"AlertmanagerUser"`
	CurrentNodeId      string
}

var cfgFile string
var logs bool

var rootCmd = &cobra.Command{
	Use:   "swarmgo",
	Short: "SwarmGo is application to create docker cluster in swarm mode",
	Long: `SwarmGo is application to create and configure docker cluster in swarm mode and install necessary tools
to work with him`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&logs, "logs", "l", false, "Redirect logs to ./logs/log.log")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".swarmgo")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
