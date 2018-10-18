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
	"log"
)

type entry struct {
	nodeName, userName string
	node               Node
}

var email string
var domain string

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
	traefikVers := clusterFile.Traefik
	clusterName := clusterFile.ClusterName
	host := firstEntry.node.Host
	traefikCompose, err := Asset("resources/traefik.yml")
	CheckErr(err)
	modifiedTraefikCompose := bytes.Replace(traefikCompose, []byte("<traefik version>"), []byte(traefikVers), -1)
	modifiedTraefikCompose = bytes.Replace(modifiedTraefikCompose, []byte("<your email>"), []byte(email), -1)
	modifiedTraefikCompose = bytes.Replace(modifiedTraefikCompose, []byte("<your domain>"), []byte(domain), -1)
	log.Println("traefik.yml modified")
	config := findSshKeysAndInitConnection(clusterName, firstEntry.userName, passToKey)
	sudoExecSshCommand(host, "docker network create -d overlay traefik", config)
	sudoExecSshCommand(host, "docker network create -d overlay webgateway", config)
	log.Println("Overlay networks created")
	execSshCommand(host, "mkdir ~/traefik", config)
	execSshCommand(host, "cat > ~/traefik/traefik.yml << EOF\n\n"+string(modifiedTraefikCompose)+"\nEOF", config)
	log.Println("traefik.yml written to host")
	sudoExecSshCommand(host, "docker stack deploy -c traefik/traefik.yml traefik", config)
	log.Println("traefik deployed")
}

func init() {
	rootCmd.AddCommand(traefikCmd)
	traefikCmd.Flags().StringVarP(&email, "email", "e", "", "Email for registration to Let's Encrypt")
	traefikCmd.Flags().StringVarP(&domain, "domain", "d", "", "Domain name for SSL certificate")
	traefikCmd.MarkFlagRequired("email")
	traefikCmd.MarkFlagRequired("domain")
}
