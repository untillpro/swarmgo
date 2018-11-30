/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cmd

import (
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"html/template"
	"io/ioutil"
	"log"
	"path/filepath"
)

const (
	traefikComposeFileName     = "traefik.yml"
	traefikTestComposeFileName = "traefik-http.yml"
	consulComposeFileName      = "consul-cluster.yml"
)

type entry struct {
	nodeName, userName string
	node               node
}

type consul struct {
	Bootstrap uint8
}

// traefikCmd represents the traefik command
var traefikCmd = &cobra.Command{
	Use:   "traefik",
	Short: "Install traefik with let's encrypt and consul on swarm cluster",
	Long:  `Install traefik with let's encrypt and consul on swarm cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		if logs {
			f := redirectLogs()
			defer f.Close()
		}
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
		nodes := getNodesFromYml(getCurrentDir())
		var bootstrap uint8
		for _, node := range nodes {
			if node.SwarmMode == manager {
				bootstrap++
			}
		}
		var bootstrapConsul consul
		if bootstrap >= 3 {
			bootstrapConsul.Bootstrap = 3
		} else {
			bootstrapConsul.Bootstrap = 1
		}
		deployConsul(bootstrapConsul, passToKey, clusterFile, firstEntry)
		deployTraefik(passToKey, clusterFile, firstEntry)
		for i, node := range nodes {
			if node.SwarmMode == leader {
				nodes[i].Traefik = true
			}
		}
		marshaledNode, err := yaml.Marshal(&nodes)
		CheckErr(err)
		nodesFilePath := filepath.Join(getCurrentDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		CheckErr(err)
	},
}

func deployConsul(consul consul, passToKey string, clusterFile *clusterFile, firstEntry *entry) {

}

func applyClusterFileTemplateToFile(filePath string, clusterFile *clusterFile) *bytes.Buffer {
	t, err := template.ParseFiles(filePath)
	var tmplBuffer bytes.Buffer
	err = t.Execute(&tmplBuffer, clusterFile)
	CheckErr(err)
	return &tmplBuffer
}

func deployTraefik(passToKey string, clusterFile *clusterFile, firstEntry *entry) {
	clusterName := clusterFile.ClusterName
	host := firstEntry.node.Host
	var traefikComposeName string
	if clusterFile.ACMEEnabled {
		traefikComposeName = traefikComposeFileName
		if len(clusterFile.Domain) == 0 || len(clusterFile.Email) == 0 {
			log.Fatal("For traefik with ACME need to specify your docker domain and email to register on letsencrypt")
		}
	} else {
		traefikComposeName = traefikTestComposeFileName
	}
	tmplBuffer := applyClusterFileTemplateToFile(filepath.Join(getCurrentDir(), traefikComposeName), clusterFile)
	log.Println("traefik.yml modified")
	config := findSSHKeysAndInitConnection(clusterName, firstEntry.userName, passToKey)
	sudoExecSSHCommand(host, "docker network create -d overlay traefik || true", config)
	sudoExecSSHCommand(host, "docker network create -d overlay webgateway || true", config)
	log.Println("overlay networks created")
	execSSHCommand(host, "mkdir -p ~/traefik", config)
	execSSHCommand(host, "cat > ~/traefik/traefik.yml << EOF\n\n"+tmplBuffer.String()+"\nEOF", config)
	log.Println("traefik.yml written to host")
	sudoExecSSHCommand(host, "docker stack deploy -c traefik/traefik.yml traefik", config)
	log.Println("traefik deployed")
}

func init() {
	rootCmd.AddCommand(traefikCmd)
}
