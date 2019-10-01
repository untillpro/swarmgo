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
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"gopkg.in/yaml.v2"
)

const (
	encryptedFlag                 = " --opt encrypted"
	traefikFolderName             = "traefik/"
	consulFolderName              = traefikFolderName + "consul/"
	traefikComposeFileName        = traefikFolderName + "traefik-consul.yml"
	traefikTestComposeFileName    = traefikFolderName + "traefik-http.yml"
	traefikStoreConfigFileName    = traefikFolderName + "storeconfig.yml"
	consulOneComposeFileName      = consulFolderName + "consul-one.yml"
	consulThreeComposeFileName    = consulFolderName + "consul-three.yml"
	consulOneServerConfFileName   = consulFolderName + "server/conf.json"
	consulThreeServerConfFileName = consulFolderName + "server/conf3.json"
	consulOneAgentConfFileName    = consulFolderName + "agent/conf.json"
	consulThreeAgentConfFileName  = consulFolderName + "agent/conf3.json"
)

type entry struct {
	nodeName, userName string
	node               node
}

type managerNodes struct {
	Consul  string
	NodeID1 string
	NodeID2 string
	NodeID3 string
}

var encrypted = ""

// traefikCmd represents the traefik command
var traefikCmd = &cobra.Command{
	Use:   "traefik",
	Short: "Install traefik with let's encrypt and consul on swarm cluster",
	Long:  `Install traefik with let's encrypt and consul on swarm cluster`,
	Run: loggedCmd(func(args []string) {
		checkSSHAgent()
		firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
		nodes := getNodesFromYml(getWorkingDir())
		host := firstEntry.node.Host
		client := getSSHClient(clusterFile)
		if clusterFile.EncryptSwarmNetworks {
			encrypted = encryptedFlag
		}
		client.ExecOrExit(host, "sudo docker network create -d overlay"+encrypted+" traefik")

		var traefikComposeName string
		if clusterFile.ACMEEnabled {
			traefikComposeName = traefikComposeFileName
			gc.Info("Traefik in production mode will be deployed")
			if len(clusterFile.Domain) == 0 || len(clusterFile.Email) == 0 {
				gc.Fatal("For traefik with ACME need to specify your docker domain and email to register on letsencrypt")
			}
			deployConsul(nodes, clusterFile, host, client)
			storeTraefikConfigToConsul(clusterFile, host, client)
			deployTraefikSSL(clusterFile, host, client)
		} else {
			client.ExecOrExit(host, "mkdir -p ~/"+traefikFolderName)
			traefikComposeName = traefikTestComposeFileName
			gc.Info("Traefik in test mode (in localhost) will be deployed")
			deployTraefik(clusterFile, host, traefikComposeName, client)
		}
		for i, node := range nodes {
			if node.SwarmMode == leader {
				nodes[i].Traefik = true
			}
		}
		marshaledNode, err := yaml.Marshal(&nodes)
		gc.ExitIfError(err)
		nodesFilePath := filepath.Join(getWorkingDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		gc.Info("Nodes written in file")
		gc.ExitIfError(err)
	}),
}

func storeTraefikConfigToConsul(clusterFile *clusterFile, host string, client *SSHClient) {
	gc.Info("Traefik store config started")
	traefikStoreConfig := executeTemplateToFile(filepath.Join(getSourcesDir(), traefikStoreConfigFileName), clusterFile)

	client.ExecOrExit(host, "mkdir -p ~/"+traefikFolderName)
	client.ExecOrExit(host, "cat > ~/"+traefikStoreConfigFileName+" << EOF\n\n"+traefikStoreConfig.String()+"\nEOF")
	client.ExecOrExit(host, "sudo docker stack deploy -c "+traefikStoreConfigFileName+" traefik")

	gc.Info("Traefik configs stored in consul")
}

func deployConsul(nodes []node, clusterFile *clusterFile, host string, client *SSHClient) {
	gc.Info("Consul deployment started")
	var bootstrap uint8
	for _, node := range nodes {
		if node.SwarmMode == manager || node.SwarmMode == leader {
			bootstrap++
		}
	}
	var nodesForConsul managerNodes
	nodesForConsul.Consul = clusterFile.Consul
	var consulAgentConfFileName, consulServerConfFileName, consulComposeFileName string
	managerIDs := client.ExecOrExit(host, "sudo docker node ls -q -f role=manager")
	managers := strings.Split(strings.Trim(managerIDs, "\r\n "), "\n")
	if bootstrap >= 3 {
		nodesForConsul.NodeID1 = managers[0]
		nodesForConsul.NodeID2 = managers[1]
		nodesForConsul.NodeID3 = managers[2]
		consulAgentConfFileName = consulThreeAgentConfFileName
		consulServerConfFileName = consulThreeServerConfFileName
		consulComposeFileName = consulThreeComposeFileName
	} else {
		nodesForConsul.NodeID1 = managers[0]
		consulAgentConfFileName = consulOneAgentConfFileName
		consulServerConfFileName = consulOneServerConfFileName
		consulComposeFileName = consulOneComposeFileName
	}
	gc.Info(fmt.Sprintf("Num of managers: %v", bootstrap))
	consulAgentConf, err := ioutil.ReadFile(filepath.Join(getWorkingDir(), consulAgentConfFileName))
	gc.ExitIfError(err)
	consulServerConf, err := ioutil.ReadFile(filepath.Join(getWorkingDir(), consulServerConfFileName))
	gc.ExitIfError(err)
	consulCompose := executeTemplateToFile(filepath.Join(getWorkingDir(), consulComposeFileName), nodesForConsul)
	gc.Info("Consul configs modified")
	client.ExecOrExit(host, "mkdir -p ~/"+consulFolderName+"agent")
	client.ExecOrExit(host, "mkdir -p ~/"+consulFolderName+"server")
	client.ExecOrExit(host, "cat > ~/"+consulAgentConfFileName+" << EOF\n\n"+string(consulAgentConf)+"\nEOF")
	client.ExecOrExit(host, "cat > ~/"+consulServerConfFileName+" << EOF\n\n"+string(consulServerConf)+"\nEOF")
	client.ExecOrExit(host, "cat > ~/"+consulComposeFileName+" << EOF\n\n"+consulCompose.String()+"\nEOF")

	gc.Info("Consul configs written to host")
	client.ExecOrExit(host, "sudo docker stack deploy -c "+consulComposeFileName+" traefik")
	gc.Info("Consul deployed, wait for consul sync")
	waitSuccessOrFailAfterTimer(host, "Synced node info", "Consul synced",
		"Consul doesn't sync in five minutes, deployment stopped", "sudo docker service logs traefik_consul_main_server1",
		5, client)
}

func executeTemplateToFile(filePath string, tmplExecutor interface{}) *bytes.Buffer {
	t, err := template.ParseFiles(filePath)
	gc.ExitIfError(err)
	var tmplBuffer bytes.Buffer
	gc.ExitIfError(t.Execute(&tmplBuffer, tmplExecutor))
	return &tmplBuffer
}

func deployTraefik(clusterFile *clusterFile, host, traefikComposeName string, client *SSHClient) {
	tmplBuffer := executeTemplateToFile(filepath.Join(getSourcesDir(), traefikComposeName), clusterFile)
	gc.Info("traefik.yml modified")
	client.ExecOrExit(host, "sudo docker network create -d overlay"+encrypted+" webgateway")
	gc.Info("webgateway networks created")
	client.ExecOrExit(host, "cat > ~/"+traefikFolderName+"traefik.yml << EOF\n\n"+tmplBuffer.String()+"\nEOF")
	client.ExecOrExit(host, "sudo docker stack deploy -c "+traefikFolderName+"traefik.yml traefik")
}

func deployTraefikSSL(clusterFile *clusterFile, host string, client *SSHClient) {
	deployTraefik(clusterFile, host, traefikComposeFileName, client)
	gc.Doing("Waiting for certs")
	waitSuccessOrFailAfterTimer(host, "Server responded with a certificate", "Cert received",
		"Cert doesn't received in five minutes, deployment stopped",
		"sudo docker service logs traefik_traefik", 3, client)
	gc.Info("traefik deployed")
}

func waitSuccessOrFailAfterTimer(host, success, logSuccess, logFail, cmd string, timeBeforeFailInMinutes time.Duration,
	client *SSHClient) {
	timer := time.NewTimer(timeBeforeFailInMinutes * time.Minute)
	doneChan := make(chan struct{})
	go func() {
		for true {
			time.Sleep(10 * time.Second)
			out := client.ExecOrExit(host, cmd)
			if strings.Contains(out, success) {
				doneChan <- struct{}{}
				break
			}
		}
	}()
	select {
	case <-doneChan:
		gc.Info(logSuccess)
	case <-timer.C:
		close(doneChan)
		gc.Fatal(logFail)
	}
	close(doneChan)
	timer.Stop()
}
