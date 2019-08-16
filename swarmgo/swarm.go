/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

const (
	worker                = "worker"
	manager               = "manager"
	leader                = "leader"
	swarmgoConfigFileName = "swarmgo-config.yml"
)

var mode bool

// swarmCmd represents the swarm command
var swarmCmd = &cobra.Command{
	Use:   "swarm -m <Alias1> <Alias2> or swarm without params(you should create one manager before do that)",
	Short: "swarm -m installs managers on given node, swarm installs workers",
	Long: `swarm with -m installs swarm manager nodes on given Aliases, swarm installs swarm workers on other nodes in
 cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		initCommand("swarm")
		defer finitCommand()
		clusterFile := unmarshalClusterYml()
		// TODO mode ???
		if mode && len(args) == 0 {
			gc.Fatal("Need at least one node alias")
		}
		nodesFromYml := getNodesFromYml(getWorkingDir())
		if len(nodesFromYml) == 0 {
			gc.Fatal("Can't find nodes from nodes.yml. Add some nodes first")
		}
		nodeHostAndNode := make(map[string]node)
		for _, value := range nodesFromYml {
			nodeHostAndNode[value.Host] = value
		}
		clusterLeaderNode, clusterManagerNodes, clusterWorkerNodes := getHostsFromNodesGroupingBySwarmModeValue(nodesFromYml)
		if clusterLeaderNode == (node{}) {
			if !mode {
				gc.Fatal("Use `-manager` flag to init swarm")
			}
			if len(args) == 0 {
				gc.Fatal("Need to pass at least one alias to init swarm")
			}
		}
		passToKey := readKeyPassword()
		nodesWithoutSwarm := make([]node, 0, len(nodesFromYml))
		for _, nodeFromYml := range nodesFromYml {
			if nodeFromYml == clusterLeaderNode || containsNode(clusterManagerNodes, nodeFromYml) ||
				containsNode(clusterWorkerNodes, nodeFromYml) {
				if contains(args, nodeFromYml.Alias) {
					gc.Info(nodeFromYml.Alias + " already in swarm")
				}
				continue
			}
			if mode {
				if ok := contains(args, nodeFromYml.Alias); !ok {
					continue
				}
			}
			nodesWithoutSwarm = append(nodesWithoutSwarm, nodeFromYml)
		}
		if len(nodesWithoutSwarm) == 0 {
			gc.Fatal("All nodes already in swarm")
		}
		var nodeVar node
		if clusterLeaderNode == (node{}) {
			nodeVar, nodesWithoutSwarm = initSwarm(nodesWithoutSwarm, args, passToKey,
				clusterFile)
			nodeHostAndNode[nodeVar.Host] = nodeVar
			clusterLeaderNode = nodeVar
		}
		var channelForNodes = make(chan nodeAndError)
		for _, currentNode := range nodesWithoutSwarm {
			go func(nodeVar node, passToKey string) {
				nodeFromGoroutine, err := joinToSwarm(nodeVar, clusterLeaderNode.Host, passToKey, clusterFile)
				nodeFromFunc := nodeAndError{
					nodeFromGoroutine,
					err,
				}
				channelForNodes <- nodeFromFunc
			}(currentNode, passToKey)
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
		CheckErr(err)
		nodesFilePath := filepath.Join(getWorkingDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		CheckErr(err)
	},
}

func getToken(mode, host string, config *ssh.ClientConfig) string {
	output := sudoExecSSHCommand(host, "docker swarm join-token "+mode, config)
	return strings.Trim(substringAfterIncludeValue(output, "docker swarm join"), "\n ")
}

func reloadUfwAndDocker(host string, config *ssh.ClientConfig) error {
	gc.Info("Restarting ufw...")
	_, err := sudoExecSSHCommandWithoutPanic(host, "ufw reload", config)
	if err != nil {
		return err
	}
	gc.Info("Ufw restarted!")
	return nil
}

func initSwarm(nodes []node, args []string,
	passToKey string, file *clusterFile) (node, []node) {
	var alias string
	alias = args[0]
	node, index := findNodeByAliasFromNodesYml(alias, nodes)
	host := node.Host
	config := findSSHKeysAndInitConnection(passToKey, file)
	err := configUfwToWorkInSwarmMode(host, config)
	CheckErr(err)
	gc.Info("Starting swarm initialization...")
	sudoExecSSHCommand(host, "ufw allow 2377/tcp", config)
	err = reloadUfwAndDocker(host, config)
	CheckErr(err)
	sudoExecSSHCommand(host, "docker swarm init --advertise-addr "+host, config)
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

func configUfwToWorkInSwarmMode(host string, config *ssh.ClientConfig) error {
	logWithPrefix(host, "Configuring ufw to work with swarm...")
	_, err := sudoExecSSHCommandWithoutPanic(host, "ufw allow 22/tcp", config)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "ufw allow 2376/tcp", config)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "ufw allow 7946/tcp", config)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "ufw allow 7946/udp", config)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "ufw allow 4789/udp", config)
	if err != nil {
		return err
	}
	logWithPrefix(host, "Ufw configured")
	return nil
}

func joinToSwarm(node node, leaderHost, passToKey string, file *clusterFile) (node, error) {
	host := node.Host
	config := findSSHKeysAndInitConnection(passToKey, file)
	err := configUfwToWorkInSwarmMode(host, config)
	if err != nil {
		return node, err
	}
	var token string
	if mode {
		_, err = sudoExecSSHCommandWithoutPanic(host, "ufw allow 2377/tcp", config)
		if err != nil {
			return node, err
		}
		token = getToken("manager", leaderHost, config)
		node.SwarmMode = manager
	} else {
		token = getToken("worker", leaderHost, config)
		node.SwarmMode = worker
	}
	err = reloadUfwAndDocker(host, config)
	if err != nil {
		node.SwarmMode = ""
		return node, err
	}
	gc.Doing("Joining " + host + " to swarm")
	// ">" is used to avoid logging
	_, err = execSSHCommandWithoutPanic(host, ">sudo "+token, config)
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
	if len(nodesFromYml) == 0 {
		gc.Fatal("Can't find nodes from nodes.yml. Add some nodes first")
	}
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
