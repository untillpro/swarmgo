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
	"path/filepath"
	"strings"

	gc "github.com/untillpro/gochips"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

const (
	eLKPrefix          = "elk"
	eLKComposeFileName = eLKPrefix + "/elk.yml"
)

// eLKCmd represents the elastic command
var eLKCmd = &cobra.Command{
	Use:   "elk",
	Short: "Deploy ELK stack",
	Long:  `Deploys Elasticsearch cluster with 3 nodes, Logstash replica, Filebeat on all nodes and single Kibana`,
	Run: loggedCmd(func(cmd *cobra.Command, args []string) {
		checkSSHAgent()
		firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
		kibanaUser := readPasswordPrompt("Kibana login")
		kibanaPass := readPasswordPrompt("Kibana password")
		kibanaHashedPass := strings.Replace(hashPassword(kibanaPass), "$", "\\$\\$", -1)
		clusterFile.KibanaCreds = fmt.Sprintf("%s:%s", kibanaUser, kibanaHashedPass)
		if !firstEntry.node.Traefik {
			gc.Fatal("Need to deploy traefik before elk deploy")
		}
		deployELKStack(clusterFile, firstEntry)
	}),
}

func deployELKStack(clusterFile *clusterFile, firstEntry *entry) {
	host := firstEntry.node.Host
	client := getSSHClient(clusterFile)
	forCopy := infoForCopy{
		host:   firstEntry.node.Host,
		client: client,
	}
	gc.Info("Installing dos2unix")
	client.ExecOrExit(host, "sudo apt-get install dos2unix")
	curDir := getSourcesDir()
	copyToHost(&forCopy, filepath.ToSlash(filepath.Join(curDir, eLKPrefix)))
	appliedBuffer := executeTemplateToFile(eLKComposeFileName, clusterFile)
	client.ExecOrExit(host, "cat > ~/"+eLKComposeFileName+" << EOF\n\n"+
		appliedBuffer.String()+"\nEOF")
	gc.Info(eLKComposeFileName, "applied by template")
	gc.Info("Increasing vm.max_map_count")
	increaseVMMaxMapCount(clusterFile)
	gc.Info("Increased")
	gc.Info("Deploying ELK")
	client.ExecOrExit(host, "sudo docker stack deploy -c "+eLKComposeFileName+" elk")
	gc.Info("ELK deployed")
}

func increaseVMMaxMapCount(clusterFile *clusterFile) {
	nodesFromYml := getNodesFromYml(getWorkingDir())
	doneChannel := make(chan interface{})
	for _, value := range nodesFromYml {
		go func(node node) {
			client := getSSHClient(clusterFile)
			_, err := client.Exec(node.Host, "sudo sysctl -w vm.max_map_count=262144")
			if err != nil {
				doneChannel <- err
			}
			doneChannel <- struct{}{}
		}(value)
	}
	errors := make([]error, 0, len(nodesFromYml))
	for range nodesFromYml {
		out := <-doneChannel
		switch out.(type) {
		case error:
			errors = append(errors, out.(error))
		}
	}
	if len(errors) != 0 {
		gc.Fatal(errors)
	}
	close(doneChannel)
}

func hashPassword(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	gc.ExitIfError(err)
	return string(hash)
}
