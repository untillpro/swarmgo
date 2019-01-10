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
	"log"
	"path/filepath"

	"github.com/spf13/cobra"
)

const eLKComposeFileName = "elk.yml"

// eLKCmd represents the elastic command
var eLKCmd = &cobra.Command{
	Use:   "elk",
	Short: "Deploy ELK stack",
	Long: `Deploys Elasticsearch cluster with 3 nodes, Logstash replica, Filebeat on all nodes and single Kibana`,
	Run: func(cmd *cobra.Command, args []string) {
		if logs {
			f := redirectLogs()
			defer func() {
				if err := f.Close(); err != nil {
					log.Println("Error closing the file: ", err.Error())
				}
			}()
		}
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
		if !firstEntry.node.Traefik {
			log.Fatal("Need to deploy traefik before elk deploy")
		}
		deployELKStack(passToKey, clusterFile, firstEntry)
	},
}

func deployELKStack(passToKey string, clusterFile *clusterFile, firstEntry *entry) {
	host := firstEntry.node.Host
	config := findSSHKeysAndInitConnection(clusterFile.ClusterName, firstEntry.userName, passToKey)
	forCopy := infoForCopy{
		firstEntry,
		config,
		clusterFile,
	}
	log.Println("Trying to install dos2unix")
	sudoExecSSHCommand(host, "apt-get install dos2unix", config)
	relativePaths := [3]string{"filebeat", "logstash", eLKComposeFileName}
	curDir := getCurrentDir()
	for _, relativePath := range relativePaths {
		copyToHost(&forCopy, filepath.ToSlash(filepath.Join(curDir, relativePath)))
	}
	filesToApplyTemplate := [1]string{eLKComposeFileName}
	for _, fileToApplyTemplate := range filesToApplyTemplate {
		appliedBuffer := applyExecutorToTemplateFile(fileToApplyTemplate, clusterFile)
		execSSHCommand(host, "cat > ~/swarmgo/"+fileToApplyTemplate+" << EOF\n\n"+
			appliedBuffer.String()+"\nEOF", config)
		log.Println(fileToApplyTemplate, "applied by template")
	}
	log.Println("Trying to deploy ELK")
	sudoExecSSHCommand(host, "docker stack deploy -c swarmgo/elk.yml elk", config)
	log.Println("ELK deployed")
}

func init() {
	rootCmd.AddCommand(eLKCmd)
}
