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
	"golang.org/x/crypto/ssh"
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
	Run: func(cmd *cobra.Command, args []string) {
		initCommand("traefik")
		defer finitCommand()
		passToKey := readKeyPassword()
		firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
		nodes := getNodesFromYml(getWorkingDir())
		host := firstEntry.node.Host
		var config = findSSHKeysAndInitConnection(passToKey, clusterFile)
		if clusterFile.EncryptSwarmNetworks {
			encrypted = encryptedFlag
		}
		sudoExecSSHCommand(host, "docker network create -d overlay"+encrypted+" traefik || true", config)
		var traefikComposeName string
		if clusterFile.ACMEEnabled {
			traefikComposeName = traefikComposeFileName
			gc.Info("Traefik in production mode will be deployed")
			if len(clusterFile.Domain) == 0 || len(clusterFile.Email) == 0 {
				gc.Fatal("For traefik with ACME need to specify your docker domain and email to register on letsencrypt")
			}
			deployConsul(nodes, clusterFile, host, config)
			storeTraefikConfigToConsul(clusterFile, host, config)
			deployTraefikSSL(clusterFile, host, config)
		} else {
			execSSHCommand(host, "mkdir -p ~/"+traefikFolderName, config)
			traefikComposeName = traefikTestComposeFileName
			gc.Info("Traefik in test mode (in localhost) will be deployed")
			deployTraefik(clusterFile, host, traefikComposeName, config)
		}
		for i, node := range nodes {
			if node.SwarmMode == leader {
				nodes[i].Traefik = true
			}
		}
		marshaledNode, err := yaml.Marshal(&nodes)
		CheckErr(err)
		nodesFilePath := filepath.Join(getWorkingDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		gc.Info("Nodes written in file")
		CheckErr(err)
	},
}

func storeTraefikConfigToConsul(clusterFile *clusterFile, host string, config *ssh.ClientConfig) {
	gc.Info("Traefik store config started")
	execSSHCommand(host, "mkdir -p ~/"+traefikFolderName, config)
	traefikStoreConfig := executeTemplateToFile(filepath.Join(getSourcesDir(), traefikStoreConfigFileName), clusterFile)
	execSSHCommand(host, "cat > ~/"+traefikStoreConfigFileName+" << EOF\n\n"+traefikStoreConfig.String()+"\nEOF", config)
	sudoExecSSHCommand(host, "docker stack deploy -c "+traefikStoreConfigFileName+" traefik", config)
	gc.Info("Traefik configs stored in consul")
}

func deployConsul(nodes []node, clusterFile *clusterFile, host string, config *ssh.ClientConfig) {
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
	managerIDs := sudoExecSSHCommand(host, "docker node ls -q -f role=manager", config)
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
	CheckErr(err)
	consulServerConf, err := ioutil.ReadFile(filepath.Join(getWorkingDir(), consulServerConfFileName))
	CheckErr(err)
	consulCompose := executeTemplateToFile(filepath.Join(getWorkingDir(), consulComposeFileName), nodesForConsul)
	gc.Info("Consul configs modified")
	execSSHCommand(host, "mkdir -p ~/"+consulFolderName+"agent", config)
	execSSHCommand(host, "mkdir -p ~/"+consulFolderName+"server", config)
	execSSHCommand(host, "cat > ~/"+consulAgentConfFileName+" << EOF\n\n"+string(consulAgentConf)+"\nEOF", config)
	execSSHCommand(host, "cat > ~/"+consulServerConfFileName+" << EOF\n\n"+string(consulServerConf)+"\nEOF", config)
	execSSHCommand(host, "cat > ~/"+consulComposeFileName+" << EOF\n\n"+consulCompose.String()+"\nEOF", config)
	gc.Info("Consul configs written to host")
	sudoExecSSHCommand(host, "docker stack deploy -c "+consulComposeFileName+" traefik", config)
	gc.Info("Consul deployed, wait for consul sync")
	waitSuccessOrFailAfterTimer(host, "Synced node info", "Consul synced",
		"Consul doesn't sync in five minutes, deployment stopped", "docker service logs traefik_consul_main_server1",
		5, config)
}

func executeTemplateToFile(filePath string, tmplExecutor interface{}) *bytes.Buffer {
	t, err := template.ParseFiles(filePath)
	CheckErr(err)
	var tmplBuffer bytes.Buffer
	err = t.Execute(&tmplBuffer, tmplExecutor)
	CheckErr(err)
	return &tmplBuffer
}

func deployTraefik(clusterFile *clusterFile, host, traefikComposeName string, config *ssh.ClientConfig) {
	tmplBuffer := executeTemplateToFile(filepath.Join(getSourcesDir(), traefikComposeName), clusterFile)
	gc.Info("traefik.yml modified")
	sudoExecSSHCommand(host, "docker network create -d overlay"+encrypted+" webgateway || true", config)
	gc.Info("webgateway networks created")
	execSSHCommand(host, "cat > ~/"+traefikFolderName+"traefik.yml << EOF\n\n"+tmplBuffer.String()+"\nEOF", config)
	sudoExecSSHCommand(host, "docker stack deploy -c "+traefikFolderName+"traefik.yml traefik", config)
}

func deployTraefikSSL(clusterFile *clusterFile, host string, config *ssh.ClientConfig) {
	deployTraefik(clusterFile, host, traefikComposeFileName, config)
	gc.Doing("Waiting for certs")
	waitSuccessOrFailAfterTimer(host, "Server responded with a certificate", "Cert received",
		"Cert doesn't received in five minutes, deployment stopped",
		"docker service logs traefik_traefik", 3, config)
	gc.Info("traefik deployed")
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
		gc.Info(logSuccess)
	case <-timer.C:
		close(doneChan)
		gc.Fatal(logFail)
	}
	close(doneChan)
	timer.Stop()
}
