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
	"github.com/spf13/cobra"
	"log"
	"strings"
)

var swarmpromCmd = &cobra.Command{
	Use:   "swarmprom",
	Short: "Create starter kti ro swarm monitoring",
	Long:  `Deploys Prometheus, SlackUrl, cAdvisor, Node Exporter, Alert Manager and Unsee to the current swarm`,
	Run: func(cmd *cobra.Command, args []string) {
		if logs {
			f := redirectLogs()
			defer f.Close()
		}
		clusterFile := unmarshalClusterYml()
		nodesFromYml := getNodesFromYml(getCurrentDir())
		if len(nodesFromYml) == 0 {
			log.Fatal("Can't find nodes from nodes.yml. Add some nodes first!")
		}
		var firstEntry *entry = nil
		var userName string
		for _, value := range nodesFromYml {
			if value.SwarmMode == 0 {
				log.Fatal("All nodes must be in swarm! Node " + value.Host + " is't part of the swarm")
			}
			if value.SwarmMode == 3 {
				fmt.Println("input user name for host " + value.Host)
				for len(userName) == 0 {
					fmt.Println("User name can't be empty!")
					userName = waitUserInput()
				}
				firstEntry = &entry{
					value.Host,
					userName,
					value,
				}
			}
		}
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		fmt.Println("Enter password for grafana admin user")
		passToGrafana := waitUserInput()
		deploySwarmprom(passToKey, passToGrafana, clusterFile, firstEntry)
	},
}

func deploySwarmprom(passToKey, passToGrafana string, clusterFile *ClusterFile, firstEntry *entry) {
	host := firstEntry.node.Host
	grafanaAdminUser := clusterFile.GrafanaAdminUser
	if grafanaAdminUser == grafanaAdminUserDefaultValue {
		log.Fatal("Need to change GrafanaAdminUser value in " + clusterFileName)
	}
	slackUrl := clusterFile.SlackUrl
	if slackUrl == slackUrlDefaultValue {
		log.Fatal("Need to change SlackUrl value in " + clusterFileName)
	}
	config := findSshKeysAndInitConnection(clusterFile.ClusterName, firstEntry.userName, passToKey)
	log.Println("Trying to clone swarmprom repository")
	execSshCommand(host, "git clone https://github.com/untillpro/swarmprom.git", config)
	log.Println("Swarmprom repository successfully cloned")
	sudoExecSshCommand(host, "docker network create -d overlay --attachable net", config)
	var strBuilder strings.Builder
	strBuilder.WriteString("ADMIN_USER=")
	strBuilder.WriteString(grafanaAdminUser)
	strBuilder.WriteString(" ADMIN_PASSWORD=")
	strBuilder.WriteString(passToGrafana)
	strBuilder.WriteString(" SLACK_URL=")
	strBuilder.WriteString(slackUrl)
	strBuilder.WriteString(" SLACK_CHANNEL=")
	strBuilder.WriteString(clusterFile.SlackChannelName)
	strBuilder.WriteString(" SLACK_USER=alertmanager docker stack deploy -c swarmprom/docker-compose.yml prom")
	log.Println("Trying to deploy swarmprom")
	sudoExecSshCommand(host, strBuilder.String(), config)
	log.Println("Swarmprom successfully deployed")
}

func init() {
	rootCmd.AddCommand(swarmpromCmd)
}
