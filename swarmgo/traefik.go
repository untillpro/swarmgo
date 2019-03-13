/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v2"
)

const (
	traefikFolderName          = "traefik/"
	consulFolderName           = traefikFolderName + "consul/"
	traefikComposeFileName     = traefikFolderName + "traefik-consul.yml"
	traefikTestComposeFileName = traefikFolderName + "traefik-http.yml"
	traefikStoreConfigFileName = traefikFolderName + "storeconfig.yml"
	consulComposeFileName      = consulFolderName + "consul-cluster.yml"
	consulServerConfFileName   = consulFolderName + "server/conf.json"
	consulAgentConfFileName    = consulFolderName + "agent/conf.json"
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
			defer func() {
				if err := f.Close(); err != nil {
					log.Println("Error closing the file: ", err.Error())
				}
			}()
		}
		passToKey := readKeyPassword()
		firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
		nodes := getNodesFromYml(getCurrentDir())
		host := firstEntry.node.Host
		var config = findSSHKeysAndInitConnection(passToKey, clusterFile)
		sudoExecSSHCommand(host, "docker network create -d overlay --opt encrypted traefik || true", config)
		var traefikComposeName string
		if clusterFile.ACMEEnabled {
			traefikComposeName = traefikComposeFileName
			log.Println("Traefik in production mode will be deployed")
			if len(clusterFile.Domain) == 0 || len(clusterFile.Email) == 0 {
				log.Fatal("For traefik with ACME need to specify your docker domain and email to register on letsencrypt")
			}
			deployConsul(nodes, clusterFile, host, config)
			storeTraefikConfigToConsul(clusterFile, host, config)
			deployTraefikSSL(clusterFile, host, config)
		} else {
			execSSHCommand(host, "mkdir -p ~/"+traefikFolderName, config)
			traefikComposeName = traefikTestComposeFileName
			log.Println("Traefik in test mode (in localhost) will be deployed")
			deployTraefik(clusterFile, host, traefikComposeName, config)
		}
		for i, node := range nodes {
			if node.SwarmMode == leader {
				nodes[i].Traefik = true
			}
		}
		marshaledNode, err := yaml.Marshal(&nodes)
		CheckErr(err)
		nodesFilePath := filepath.Join(getCurrentDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		log.Println("Nodes written in file")
		CheckErr(err)
	},
}

func storeTraefikConfigToConsul(clusterFile *clusterFile, host string, config *ssh.ClientConfig) {
	log.Println("Traefik store config started")
	execSSHCommand(host, "mkdir -p ~/"+traefikFolderName, config)
	traefikStoreConfig := applyExecutorToTemplateFile(filepath.Join(getCurrentDir(), traefikStoreConfigFileName), clusterFile)
	execSSHCommand(host, "cat > ~/"+traefikStoreConfigFileName+" << EOF\n\n"+traefikStoreConfig.String()+"\nEOF", config)
	sudoExecSSHCommand(host, "docker stack deploy -c "+traefikStoreConfigFileName+" traefik", config)
	log.Println("Traefik configs stored in consul")
}

func deployConsul(nodes []node, clusterFile *clusterFile, host string, config *ssh.ClientConfig) {
	log.Println("Consul deployment started")
	var bootstrap uint8
	for _, node := range nodes {
		if node.SwarmMode == manager || node.SwarmMode == leader {
			bootstrap++
		}
	}
	var bootstrapConsul consul
	if bootstrap >= 3 {
		bootstrapConsul.Bootstrap = 3
	} else {
		bootstrapConsul.Bootstrap = 1
	}
	log.Printf("Num of managers: %v, bootstrap expect: %v", bootstrap, bootstrapConsul.Bootstrap)
	consulAgentConf, err := ioutil.ReadFile(filepath.Join(getCurrentDir(), consulAgentConfFileName))
	CheckErr(err)
	consulServerConf := applyExecutorToTemplateFile(filepath.Join(getCurrentDir(), consulServerConfFileName), bootstrapConsul)
	consulCompose := applyExecutorToTemplateFile(filepath.Join(getCurrentDir(), consulComposeFileName), clusterFile)
	log.Println("Consul configs modified")
	execSSHCommand(host, "mkdir -p ~/"+consulFolderName+"agent", config)
	execSSHCommand(host, "mkdir -p ~/"+consulFolderName+"server", config)
	execSSHCommand(host, "cat > ~/"+consulAgentConfFileName+" << EOF\n\n"+string(consulAgentConf)+"\nEOF", config)
	execSSHCommand(host, "cat > ~/"+consulServerConfFileName+" << EOF\n\n"+consulServerConf.String()+"\nEOF", config)
	execSSHCommand(host, "cat > ~/"+consulComposeFileName+" << EOF\n\n"+consulCompose.String()+"\nEOF", config)
	log.Println("Consul configs written to host")
	sudoExecSSHCommand(host, "docker stack deploy -c "+consulComposeFileName+" traefik", config)
	log.Println("Consul deployed, wait for consul sync")
	waitSuccessOrFailAfterTimer(host, "Synced node info", "Consul synced",
		"Consul doesn't sync in five minutes, deployment stopped", "docker service logs traefik_consul_server",
		5, config)
}

func applyExecutorToTemplateFile(filePath string, tmplExecutor interface{}) *bytes.Buffer {
	t, err := template.ParseFiles(filePath)
	var tmplBuffer bytes.Buffer
	err = t.Execute(&tmplBuffer, tmplExecutor)
	CheckErr(err)
	return &tmplBuffer
}

func deployTraefik(clusterFile *clusterFile, host, traefikComposeName string, config *ssh.ClientConfig) {
	tmplBuffer := applyExecutorToTemplateFile(filepath.Join(getCurrentDir(), traefikComposeName), clusterFile)
	log.Println("traefik.yml modified")
	sudoExecSSHCommand(host, "docker network create -d overlay --opt encrypted webgateway || true", config)
	log.Println("webgateway networks created")
	execSSHCommand(host, "cat > ~/"+traefikFolderName+"traefik.yml << EOF\n\n"+tmplBuffer.String()+"\nEOF", config)
	sudoExecSSHCommand(host, "docker stack deploy -c "+traefikFolderName+"traefik.yml traefik", config)
}

func deployTraefikSSL(clusterFile *clusterFile, host string, config *ssh.ClientConfig) {
	out := sudoExecSSHCommand(host, "docker node ls --format \"{{if .Self}}{{.ID}}{{end}}\"", config)
	out = strings.Trim(out, "\n ")
	clusterFile.CurrentNodeId = out
	deployTraefik(clusterFile, host, traefikComposeFileName, config)
	waitSuccessOrFailAfterTimer(host, "Server responded with a certificate", "Cert received",
		"Cert doesn't received in five minutes, deployment stopped",
		"docker service logs traefik_traefik", 3, config)
	log.Println("traefik.yml written to host")
	sudoExecSSHCommand(host, "docker service update --constraint-rm=\"node.id == "+out+"\" traefik_traefik", config)
	log.Println("traefik deployed")
}

func waitSuccessOrFailAfterTimer(host, success, logSuccess, logFail, cmd string, timeBeforeFailInMinutes time.Duration,
	config *ssh.ClientConfig) {
	timer := time.NewTimer(timeBeforeFailInMinutes * time.Minute)
	doneChan := make(chan struct{})
	go func() {
		for true {
			time.Sleep(10 * time.Second)
			out := sudoExecSSHCommand(host, cmd, config)
			if strings.Contains(out, success) {
				doneChan <- struct{}{}
				break
			}
		}
	}()
	select {
	case <-doneChan:
		log.Println(logSuccess)
	case <-timer.C:
		close(doneChan)
		log.Fatal(logFail)
	}
	close(doneChan)
	timer.Stop()
}
