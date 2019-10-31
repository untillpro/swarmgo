/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type clusterFile struct {
	OrganizationName      string                       `yaml:"Organization"`
	ClusterName           string                       `yaml:"Cluster"`
	RootUserName          string                       `yaml:"RootUser"`
	ClusterUserName       string                       `yaml:"ClusterUser"`
	PublicKey             string                       `yaml:"PublicKey"`
	PrivateKey            string                       `yaml:"PrivateKey"`
	Docker                map[string]map[string]string `yaml:"Docker"`
	Alertmanager          string                       `yaml:"Alertmanager"`
	NodeExporter          string                       `yaml:"NodeExporter"`
	Prometheus            string                       `yaml:"Prometheus"`
	Grafana               string                       `yaml:"Grafana"`
	Traefik               string                       `yaml:"Traefik"`
	Socat                 string                       `yaml:"Socat"`
	Cadvisor              string                       `yaml:"Cadvisor"`
	Consul                string                       `yaml:"Consul"`
	ACMEEnabled           bool                         `yaml:"ACMEEnabled"`
	Domain                string                       `yaml:"Domain"`
	Email                 string                       `yaml:"Email"`
	GrafanaUser           string                       `yaml:"GrafanaUser"`
	PrometheusUser        string                       `yaml:"PrometheusUser"`
	TraefikUser           string                       `yaml:"TraefikUser"`
	ChannelName           string                       `yaml:"ChannelName"`
	AlertmanagerUser      string                       `yaml:"AlertmanagerUser"`
	Elasticsearch         string                       `yaml:"Elasticsearch"`
	Filebeat              string                       `yaml:"Filebeat"`
	Kibana                string                       `yaml:"Kibana"`
	Logstash              string                       `yaml:"Logstash"`
	Curator               string                       `yaml:"Curator"`
	EncryptSwarmNetworks  bool                         `yaml:"EncryptSwarmNetworks"`
	WebhookURL            string
	GrafanaPassword       string
	PrometheusBasicAuth   string
	AlertManagerBasicAuth string
	TraefikBasicAuth      string
	KibanaCreds           string
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

	rootCmd.AddCommand(keysCmd)
	keysCmd.Flags().StringVarP(&privateKeyArg, "private", "p", "", "Private key path")
	keysCmd.Flags().StringVarP(&publicKeyArg, "public", "u", "", "Public key path")

	rootCmd.AddCommand(agentCmd)

	rootCmd.AddCommand(addNodeCmd)
	addNodeCmd.Flags().BoolVarP(&skipSSHConfiguration, "skipssh", "s", false, "Use this option when ClusterUser already exists and SSH access is configured for on nodes being added")
	addNodeCmd.Flags().StringVarP(&rootPassword, "rootPassword", "p", "", "Specify default password. Requires sshpass on Linux and plink on Windows")

	rootCmd.AddCommand(dockerCmd)
	dockerCmd.Flags().BoolVarP(&forceUpgradeDocker, "upgrade", "u", false, "Upgrade docker to the latest version, if already installed")

	rootCmd.AddCommand(labelCmd)
	labelCmd.AddCommand(labelLsCmd)
	labelCmd.AddCommand(labelAddCmd)
	labelCmd.AddCommand(labelRmCmd)

	rootCmd.AddCommand(eLKCmd)

	rootCmd.AddCommand(swarmCmd)
	swarmCmd.Flags().BoolVarP(&mode, "manager", "m", false, "Swarm mode: m means `join-manager")

	rootCmd.AddCommand(swarmpromCmd)
	swarmpromCmd.Flags().BoolVarP(&argUpgradeSwarmprom, "update", "u", false, "Update alert manager configuration")
	swarmpromCmd.Flags().BoolVarP(&argNoAlerts, "noalerts", "n", false, "Configure no push alerts in Prometheus Alertmamnager")
	swarmpromCmd.Flags().StringVarP(&argSlackWebhookURL, "slackWebhookUrl", "s", "", "Configure Slack alerts by specifying Webhook URL")
	swarmpromCmd.Flags().StringVarP(&argGrafanaPass, "grafanaPass", "g", "", "Specify password for Grafana web-ui")
	swarmpromCmd.Flags().StringVarP(&argPrometheusPass, "prometheusPass", "p", "", "Specify password for Prometheus web-ui")
	swarmpromCmd.Flags().StringVarP(&argAlertmanagerPass, "alertmanagerPass", "a", "", "Specify password for Alertmanager web-ui")

	rootCmd.AddCommand(traefikCmd)
	traefikCmd.Flags().StringVarP(&argTraefikPass, "password", "p", "", "Specify password for Traefik dashboard web-ui")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
