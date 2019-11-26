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
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"gopkg.in/yaml.v2"
)

type consulCfg struct {
	Host       string
	LeaderHost string
}

// InstallConsul installs consul
func InstallConsul(server bool, args []string) {
	clusterFile := unmarshalClusterYml()

	nodesFromYml := getNodesFromYml(getWorkingDir())
	gc.ExitIfFalse(len(nodesFromYml) > 0, "Can't find nodes from nodes.yml. Add some nodes first")

	nodeHostAndNode := make(map[string]node)
	for _, value := range nodesFromYml {
		nodeHostAndNode[value.Host] = value
	}

	clusterLeaderNode := node{}
	if server {
		clusterLeaderNode = nodesFromYml[0] // TODO: temporary solution!
	}
	var channelForNodes = make(chan nodeAndError)
	for _, currentNode := range nodesFromYml {
		go func(nodeVar node) {
			nodeFromGoroutine, err := installConsulAtNode(nodeVar, clusterLeaderNode.Host, clusterFile, server)
			nodeFromFunc := nodeAndError{
				nodeFromGoroutine,
				err,
			}
			channelForNodes <- nodeFromFunc
		}(currentNode)
	}
	errMsgs := make([]string, 0, len(args))
	for _, key := range nodesFromYml {
		nodeWithPossibleError := <-channelForNodes
		node := nodeWithPossibleError.nodeWithPossibleError
		err := nodeWithPossibleError.err
		if nodeWithPossibleError.err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("Host: %v, returns error: %v", node.Host,
				err.Error()))
		}
		nodeHostAndNode[key.Host] = node
	}
	for _, errMsg := range errMsgs {
		gc.Info(errMsg)
	}
	close(channelForNodes)
	nodes := make([]node, len(nodeHostAndNode))
	i := 0
	for _, value := range nodeHostAndNode {
		nodes[i] = value
		i++
	}
	marshaledNode, err := yaml.Marshal(&nodes)
	gc.ExitIfError(err)

	nodesFilePath := filepath.Join(getWorkingDir(), nodesFileName)

	gc.ExitIfError(ioutil.WriteFile(nodesFilePath, marshaledNode, 0600))

	gc.ExitIfFalse(len(errMsgs) == 0, "Failed to install on some node(s)")
}

// consulCmd represents the swarm command
var consulCmd = &cobra.Command{
	Use: "consul -s <Alias1> <Alias2> installs consul in server mode (experimental)",
	Run: func(cmd *cobra.Command, args []string) {
		initCommand("consul")
		defer finitCommand()
		if mode && len(args) == 0 {
			gc.Fatal("Need at least one node alias")
		}
		checkSSHAgent()
		InstallConsul(mode, args)
	},
}

/*func findNodeByAliasFromNodesYml(alias string, nodesFromYml []node) (node, int) {
	var leaderNode node
	var index int
	for i, node := range nodesFromYml {
		if node.Alias == alias {
			leaderNode = node
			index = i
		}
	}
	if leaderNode == (node{}) {
		gc.Info("Can't find host by given alias in nodes.yml, choose it interactive")
		alias := numberHostsFromNodesFile(nodesFromYml)
		return findNodeByAliasFromNodesYml(alias, nodesFromYml)
	}
	return leaderNode, index
}

func getHostsFromNodesGroupingBySwarmModeValue(nodes []node) (node, []node, []node) {
	var clusterLeaderHost node
	var clusterManagerHosts []node
	var clusterWorkersHost []node
	for _, node := range nodes {
		if len(node.DockerVersion) == 0 {
			gc.Fatal("Need to install docker on all nodes from nodes.yml, please exec `swarmgo docker`")
		}
		switch node.SwarmMode {
		case leader:
			clusterLeaderHost = node
		case manager:
			clusterManagerHosts = append(clusterManagerHosts, node)
		case worker:
			clusterWorkersHost = append(clusterWorkersHost, node)
		}
	}
	return clusterLeaderHost, clusterManagerHosts, clusterWorkersHost
}
*/
func configUfwForConsul(host string, client *SSHClient) error {
	commands := []SSHCommand{
		SSHCommand{
			cmd:   "sudo apt-get -y install ufw",
			title: "Installing firewall",
		},
		SSHCommand{
			title: "Setting up firewall rules",
			cmd:   "sudo ufw allow 22/tcp",
			cmd2:  "sudo ufw allow 8300/tcp",
			cmd3:  "sudo ufw allow 8301/tcp",
			cmd4:  "sudo ufw allow 8301/udp",
			cmd5:  "sudo ufw allow 8302/tcp",
			cmd6:  "sudo ufw allow 8302/udp",
		},
	}

	err := sshKeyAuthCmds(host, client, commands)
	if err != nil {
		return err
	}

	logWithPrefix(host, "Firewall configured")
	return nil
}

func installConsulAtNode(node node, leaderHost string, file *clusterFile, server bool) (node, error) {

	client := getSSHClient(file)

	err := configUfwForConsul(node.Host, client)
	if err != nil {
		return node, err
	}

	cfg := consulCfg{
		Host:       node.Host,
		LeaderHost: leaderHost,
	}

	templateAndCopy(client, node.Host, "scripts/consul.service", "~/consul.service", cfg)
	templateAndCopy(client, node.Host, "scripts/consul.hcl", "~/consul.hcl", cfg)
	templateAndCopy(client, node.Host, "scripts/server.hcl", "~/server.hcl", cfg)

	consulVersion := file.Consul[7:]

	commands := []SSHCommand{
		SSHCommand{
			cmd:   fmt.Sprintf("curl --silent --remote-name https://releases.hashicorp.com/consul/%s/consul_%s_linux_amd64.zip", consulVersion, consulVersion),
			title: "Downloading consul",
		},
		SSHCommand{
			cmd:   fmt.Sprintf("unzip consul_%s_linux_amd64.zip", consulVersion),
			title: "Unzipping package",
		},
		SSHCommand{
			title: "Moving to /usr/local/bin/",
			cmd:   "sudo chown root:root consul",
			cmd2:  "sudo mv consul /usr/local/bin/",
		},
		SSHCommand{
			cmd:   "sudo useradd --system --home /etc/consul.d --shell /bin/false consul",
			title: "Creating user consul",
		},
		SSHCommand{
			title: "Creating user home dir",
			cmd:   "sudo mkdir --parents /opt/consul",
			cmd1:  "sudo chown --recursive consul:consul /opt/consul",
		},
		SSHCommand{
			cmd:   "sudo chmod 777 /etc/systemd/system",
			cmd1:  "sudo mv ~/consul.service /etc/systemd/system/consul.service",
			title: "Creating service",
		},
		SSHCommand{
			title: "Creating consul configuration file",
			cmd:   "sudo mkdir --parents /etc/consul.d",
			cmd1:  "sudo chmod 777 /etc/consul.d",
			cmd2:  "sudo mv ~/consul.hcl /etc/consul.d/consul.hcl",
		},
		SSHCommand{
			title: "Creating server cfg file",
			cmd:   "sudo mv ~/server.hcl /etc/consul.d/server.hcl",
			cmd2:  "sudo chown --recursive consul:consul /etc/consul.d",
			cmd3:  "sudo chmod 640 /etc/consul.d/server.hcl",
			cmd4:  "sudo chmod 640 /etc/consul.d/consul.hcl",
		},
		SSHCommand{
			title: "Starting consul service",
			cmd:   "sudo systemctl enable consul",
			cmd2:  "sudo systemctl start consul",
		},
	}

	err = sshKeyAuthCmds(node.Host, client, commands)
	if err != nil {
		return node, err
	}

	/*	ver, err := client.Exec(node.Host, "consul --version")
		if ver != consulVersion {
			return node, fmt.Errorf("Couldn't install consul version %s", consulVersion)
		} */

	node.SwarmMode = manager
	logWithPrefix(node.Host, node.Alias+" consul successfully deployed")
	return node, nil
}

/*
func getSwarmLeaderNodeAndClusterFile() (*entry, *clusterFile) {
	clusterFile := unmarshalClusterYml()
	nodesFromYml := getNodesFromYml(getWorkingDir())
	gc.ExitIfFalse(len(nodesFromYml) > 0, "Can't find nodes from nodes.yml. Add some nodes first")
	var firstEntry *entry
	//need to create networks in manager node
	for _, value := range nodesFromYml {
		//if value.SwarmMode == 0 {
		//	gc.Fatal("All nodes must be in swarm! Node " + value.Host + " isn't part of the swarm")
		//}
		if value.SwarmMode == leader {
			firstEntry = &entry{
				value.Host,
				clusterFile.ClusterUserName,
				value,
			}
		}
	}
	return firstEntry, clusterFile
}
*/
