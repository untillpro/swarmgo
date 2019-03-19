/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type clusterFile struct {
	OrganizationName   string `yaml:"Organization"`
	ClusterName        string `yaml:"Cluster"`
	RootUserName       string `yaml:"RootUser"`
	ClusterUserName    string `yaml:"ClusterUser"`
	PublicKey          string `yaml:"PublicKey,omitempty"`
	PrivateKey         string `yaml:"PrivateKey,omitempty"`
	Docker             string `yaml:"Docker"`
	Alertmanager       string `yaml:"Alertmanager"`
	NodeExporter       string `yaml:"NodeExporter"`
	Prometheus         string `yaml:"Prometheus"`
	Grafana            string `yaml:"Grafana"`
	Traefik            string `yaml:"Traefik"`
	Cadvisor           string `yaml:"Cadvisor"`
	Consul             string `yaml:"Consul"`
	ACMEEnabled        bool   `yaml:"ACMEEnabled"`
	Domain             string `yaml:"Domain"`
	Email              string `yaml:"Email"`
	GrafanaUser        string `yaml:"GrafanaUser"`
	GrafanaPassword    string `yaml:",omitempty"`
	PrometheusUser     string `yaml:"PrometheusUser"`
	PrometheusPassword string `yaml:",omitempty"`
	TraefikUser        string `yaml:"TraefikUser"`
	TraefikPassword    string `yaml:",omitempty"`
	WebhookURL         string `yaml:",omitempty"`
	ChannelName        string `yaml:"ChannelName"`
	AlertmanagerUser   string `yaml:"AlertmanagerUser"`
	Elasticsearch      string `yaml:"Elasticsearch"`
	Filebeat           string `yaml:"Filebeat"`
	Kibana             string `yaml:"Kibana"`
	Logstash           string `yaml:"Logstash"`
	Curator            string `yaml:"Curator"`
	CurrentNodeId      string `yaml:",omitempty"`
	KibanaCreds        string `yaml:",omitempty"`
}

var verbose bool
var logs bool

var rootCmd = &cobra.Command{
	Use:   "swarmgo",
	Short: "swarmgo creates docker cluster in swarm mode",
}

// Execute rootCmd
func Execute() {

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(addNodeCmd)
	rootCmd.AddCommand(dockerCmd)
	rootCmd.AddCommand(eLKCmd)

	rootCmd.AddCommand(swarmCmd)
	swarmCmd.Flags().BoolVarP(&mode, "manager", "m", false, "Swarm mode: m means `join-manager")

	rootCmd.AddCommand(swarmpromCmd)
	rootCmd.AddCommand(traefikCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
