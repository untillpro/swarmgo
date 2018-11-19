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
	"log"
	"path/filepath"
)

type entry struct {
	nodeName, userName string
	node               node
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
		createTraefik(passToKey, clusterFile, firstEntry)
	},
}

func applyClusterFileTemplateToFile(filePath string, clusterFile *clusterFile) *bytes.Buffer {
	tmpl, err := template.ParseFiles(filePath)
	var tmplBuffer bytes.Buffer
	err = tmpl.Execute(&tmplBuffer, clusterFile)
	CheckErr(err)
	return &tmplBuffer
}

func createTraefik(passToKey string, clusterFile *clusterFile, firstEntry *entry) {
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
	sudoExecSSHCommand(host, "docker network create -d overlay traefik", config)
	sudoExecSSHCommand(host, "docker network create -d overlay webgateway", config)
	log.Println("overlay networks created")
	execSSHCommand(host, "mkdir ~/traefik", config)
	execSSHCommand(host, "cat > ~/traefik/traefik.yml << EOF\n\n"+tmplBuffer.String()+"\nEOF", config)
	log.Println("traefik.yml written to host")
	sudoExecSSHCommand(host, "docker stack deploy -c traefik/traefik.yml traefik", config)

	log.Println("traefik deployed")
}

func init() {
	rootCmd.AddCommand(traefikCmd)
}
