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
	"strings"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"gopkg.in/yaml.v2"
)

const (
	worker  = "worker"
	manager = "manager"
	leader  = "leader"
)

var mode bool

// AddToSwarm adds nodes to swarm s
func AddToSwarm(manager bool, args []string) {
	clusterFile := unmarshalClusterYml()

	nodesFromYml := getNodesFromYml(getWorkingDir())
	gc.ExitIfFalse(len(nodesFromYml) > 0, "Can't find nodes from nodes.yml. Add some nodes first")

	nodeHostAndNode := make(map[string]node)
	for _, value := range nodesFromYml {
		nodeHostAndNode[value.Host] = value
	}
	clusterLeaderNode, clusterManagerNodes, clusterWorkerNodes := getHostsFromNodesGroupingBySwarmModeValue(nodesFromYml)
	if clusterLeaderNode == (node{}) {
		gc.ExitIfFalse(manager, "Use `-manager` flag to init swarm")
		gc.ExitIfFalse(len(args) > 0, "Need to pass at least one alias to init swarm")
	}
	nodesWithoutSwarm := make([]node, 0, len(nodesFromYml))
	for _, nodeFromYml := range nodesFromYml {
		if nodeFromYml == clusterLeaderNode || containsNode(clusterManagerNodes, nodeFromYml) ||
			containsNode(clusterWorkerNodes, nodeFromYml) {
			if contains(args, nodeFromYml.Alias) {
				gc.Info(nodeFromYml.Alias + " already in swarm")
			}
			continue
		}
		if manager {
			if ok := contains(args, nodeFromYml.Alias); !ok {
				continue
			}
		}
		nodesWithoutSwarm = append(nodesWithoutSwarm, nodeFromYml)
	}
	gc.ExitIfFalse(len(nodesWithoutSwarm) > 0, "All nodes already in swarm")

	var nodeVar node
	if clusterLeaderNode == (node{}) {
		nodeVar, nodesWithoutSwarm = initSwarm(nodesWithoutSwarm, args,
			clusterFile)
		nodeHostAndNode[nodeVar.Host] = nodeVar
		clusterLeaderNode = nodeVar
	}
	var channelForNodes = make(chan nodeAndError)
	for _, currentNode := range nodesWithoutSwarm {
		go func(nodeVar node) {
			nodeFromGoroutine, err := joinToSwarm(nodeVar, clusterLeaderNode.Host, clusterFile, manager)
			nodeFromFunc := nodeAndError{
				nodeFromGoroutine,
				err,
			}
			channelForNodes <- nodeFromFunc
		}(currentNode)
	}
	errMsgs := make([]string, 0, len(args))
	for _, key := range nodesWithoutSwarm {
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

// swarmCmd represents the swarm command
var swarmCmd = &cobra.Command{
	Use:   "swarm -m <Alias1> <Alias2> or swarm without params (you should create one manager before doing that)",
	Short: "swarm -m installs managers on given node, swarm installs workers",
	Long: `swarm with -m installs swarm manager nodes on given Aliases, swarm installs swarm workers on other nodes in
 cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		initCommand("swarm")
		defer finitCommand()
		if mode && len(args) == 0 {
			gc.Fatal("Need at least one node alias")
		}
		checkSSHAgent()
		AddToSwarm(mode, args)
	},
}

func getToken(mode, host string, client *SSHClient, targetHost string) (string, error) {
	output, err := client.Exec(host, "$sudo docker swarm join-token "+mode) // "$" prefix masks output
	return strings.Trim(substringAfterIncludeValue(output, "docker swarm join --advertise-addr \""+targetHost+"\""), "\n "), err
}

func reloadUfwAndDocker(host string, client *SSHClient) error {
	logWithPrefix(host, "Restarting ufw...")
	_, err := client.Exec(host, "sudo ufw reload")
	if err != nil {
		return err
	}
	logWithPrefix(host, "Ufw restarted!")
	return nil
}

func initSwarm(nodes []node, args []string, file *clusterFile) (node, []node) {
	var alias string
	alias = args[0]
	node, index := findNodeByAliasFromNodesYml(alias, nodes)
	host := node.Host
	client := getSSHClient(file)
	err := configUfwToWorkInSwarmMode(host, client)
	gc.ExitIfError(err)
	gc.Info("Starting swarm initialization...")
	_, err = client.Exec(host, "sudo ufw allow 2377/tcp")
	gc.ExitIfError(err)

	err = reloadUfwAndDocker(host, client)
	gc.ExitIfError(err)

	_, err = client.Exec(host, "sudo docker swarm init --advertise-addr "+host)
	nodes = append(nodes[:index], nodes[index+1:]...)
	node.SwarmMode = leader
	gc.Info("Swarm initialized, leader node is " + alias)
	return node, nodes
}

func findNodeByAliasFromNodesYml(alias string, nodesFromYml []node) (node, int) {
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

func configUfwToWorkInSwarmMode(host string, client *SSHClient) error {
	commands := []SSHCommand{
		SSHCommand{
			cmd:   "sudo apt-get -y install ufw",
			title: "Installing ufw",
		},
		SSHCommand{
			cmd:   "sudo ufw allow 22/tcp",
			title: "Adding ufw rule 22/tcp",
		},
		SSHCommand{
			cmd:   "sudo ufw allow 2376/tcp",
			title: "Adding ufw rule 2376/tcp",
		},
		SSHCommand{
			cmd:   "sudo ufw allow 7946/tcp",
			title: "Adding ufw rule 7946/tcp",
		},
		SSHCommand{
			cmd:   "sudo ufw allow 7946/udp",
			title: "Adding ufw rule 7946/udp",
		},
		SSHCommand{
			cmd:   "sudo ufw allow 4789/udp",
			title: "Adding ufw rule 4789/udp",
		},
		SSHCommand{
			cmd:   "sudo ufw allow proto esp from any",
			title: "Adding ufw rule for ESP proto",
		},
	}

	err := sshKeyAuthCmds(host, client, commands)
	if err != nil {
		return err
	}

	logWithPrefix(host, "Ufw configured")
	return nil
}

func joinToSwarm(node node, leaderHost string, file *clusterFile, mgr bool) (node, error) {

	client := getSSHClient(file)

	err := configUfwToWorkInSwarmMode(node.Host, client)
	if err != nil {
		return node, err
	}
	var token string
	if mgr {
		_, err = client.Exec(node.Host, "sudo ufw allow 2377/tcp")
		if err != nil {
			return node, err
		}
		token, err = getToken("manager", leaderHost, client, node.Host)
		if err != nil {
			return node, err
		}
		node.SwarmMode = manager
	} else {
		token, err = getToken("worker", leaderHost, client, node.Host)
		if err != nil {
			return node, err
		}
		node.SwarmMode = worker
	}
	err = reloadUfwAndDocker(node.Host, client)
	if err != nil {
		node.SwarmMode = ""
		return node, err
	}
	gc.Doing("Joining " + node.Host + " to swarm")
	// "!" is used to avoid logging
	_, err = client.Exec(node.Host, "!sudo "+token)
	if err != nil {
		node.SwarmMode = ""
		return node, err
	}
	logWithPrefix(node.Host, node.Alias+" successfully joined swarm")
	return node, nil
}

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
