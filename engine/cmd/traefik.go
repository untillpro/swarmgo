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
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type entry struct {
	nodeName, userName string
	node               Node
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
		clusterFile := unmarshalClusterYml()
		nodesFromYml := getNodesFromYml(getCurrentDir())
		if len(nodesFromYml) == 0 {
			log.Fatal("Can't find nodes from nodes.yml. Add some nodes first!")
		}
		var firstEntry *entry = nil
		//need to create networks in manager node
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
		createTraefik(passToKey, clusterFile, firstEntry)
	},
}

func createTraefik(passToKey string, clusterFile *ClusterFile, firstEntry *entry) {
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
	traefikComposeFile, err := os.Open(filepath.Join(getCurrentDir(), traefikComposeName))
	CheckErr(err)
	traefikComposeFileContent, err := ioutil.ReadAll(traefikComposeFile)
	CheckErr(err)
	tmpl, err := template.New("traefik").Parse(string(traefikComposeFileContent))
	var tmplBuffer bytes.Buffer
	err = tmpl.Execute(&tmplBuffer, clusterFile)
	CheckErr(err)
	log.Println("traefik.yml modified")
	config := findSshKeysAndInitConnection(clusterName, firstEntry.userName, passToKey)
	sudoExecSSHCommand(host, "docker network create -d overlay traefik", config)
	sudoExecSSHCommand(host, "docker network create -d overlay webgateway", config)
	log.Println("Overlay networks created")
	execSSHCommand(host, "mkdir ~/traefik", config)
	execSSHCommand(host, "cat > ~/traefik/traefik.yml << EOF\n\n"+tmplBuffer.String()+"\nEOF", config)
	log.Println("traefik.yml written to host")
	sudoExecSSHCommand(host, "docker stack deploy -c traefik/traefik.yml traefik", config)
	log.Println("traefik deployed")
}

func init() {
	rootCmd.AddCommand(traefikCmd)
}
