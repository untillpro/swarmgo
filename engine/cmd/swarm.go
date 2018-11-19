/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
)

var mode bool
var swarmChan = make(chan node)

// swarmCmd represents the swarm command
var swarmCmd = &cobra.Command{
	Use:   "swarm -m <Alias1> <Alias2> or swarm without params",
	Short: "swarm -m installs managers on given node, swarm installs workers",
	Long: `swarm with -m installs swarm manager nodes on given Aliases, swarm installs swarm workers on other nodes in
 cluster`,
	Run: func(cmd *cobra.Command, args []string) {
		if logs {
			f := redirectLogs()
			defer f.Close()
		}
		clusterFile := unmarshalClusterYml()
		if mode && len(args) == 0 {
			//if we want to choose alias interactive
			//alias = chooseHostFromNodesYml(getCurrentDir())
			log.Fatal("Need to pass at least one alias to init swarm!")
		}
		nodesFromYml := getNodesFromYml(getCurrentDir())
		if len(nodesFromYml) == 0 {
			log.Fatal("Can't find nodes from nodes.yml. Add some nodes first!")
		}
		nodeHostAndNode := make(map[string]node)
		for _, value := range nodesFromYml {
			nodeHostAndNode[value.Host] = value
		}
		clusterLeaderNode, clusterManagerNodes, clusterWorkerNodes := getHostsFromNodesGroupingBySwarmModeValue(nodesFromYml)
		if clusterLeaderNode == (node{}) {
			if !mode {
				log.Fatal("Need to specify -manager to init swarm!")
			}
			if len(args) == 0 {
				log.Fatal("Need to pass at least one alias to init swarm!")
			}
		}
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		nodeAndUserName := make(map[node]string)
		for _, nodeFromYml := range nodesFromYml {
			if nodeFromYml == clusterLeaderNode || containsNode(clusterManagerNodes, nodeFromYml) ||
				containsNode(clusterWorkerNodes, nodeFromYml) {
				log.Println(nodeFromYml.Alias + " already in swarm!")
				continue
			}
			var userName string
			if mode {
				if ok := contains(args, nodeFromYml.Alias); !ok {
					continue
				}
			}
			fmt.Println("input user name for host " + nodeFromYml.Host)
			for len(userName) == 0 {
				fmt.Println("User name can't be empty!")
				userName = waitUserInput()
			}
			nodeAndUserName[nodeFromYml] = userName
		}
		if len(nodeAndUserName) == 0 {
			log.Fatal("All nodes already in swarm")
		}
		var nodeVar node
		if clusterLeaderNode == (node{}) {
			nodeVar, nodeAndUserName = initSwarm(nodesFromYml, nodeAndUserName, args, passToKey, clusterFile.ClusterName)
			nodeHostAndNode[nodeVar.Host] = nodeVar
		}
		for key, value := range nodeAndUserName {
			go func(nodeVar node, userName, passToKey string) {
				joinToSwarm(nodeVar, clusterLeaderNode.Host, userName, passToKey, clusterFile.ClusterName)
			}(key, value, passToKey)
		}
		for  key := range nodeAndUserName {
			nodeHostAndNode[key.Host] = <-swarmChan
		}
		close(swarmChan)
		nodes := make([]node, len(nodeHostAndNode))
		i := 0
		for _, value := range nodeHostAndNode {
			nodes[i] = value
			i++
		}
		marshaledNode, err := yaml.Marshal(&nodes)
		CheckErr(err)
		nodesFilePath := filepath.Join(getCurrentDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		CheckErr(err)
	},
}

func getToken(mode, host string, config *ssh.ClientConfig) string {
	output := sudoExecSSHCommand(host, "docker swarm join-token "+mode, config)
	return strings.Trim(substringAfterIncludeValue(output, "docker swarm join"), "\n ")
}

func reloadUfwAndDocker(host string, config *ssh.ClientConfig) {
	log.Println("Restarting ufw and docker...")
	sudoExecSSHCommand(host, "ufw reload", config)
	sudoExecSSHCommand(host, "systemctl restart docker", config)
	log.Println("Ufw and docker restarted!")
}

func initSwarm(nodesFromYml []node, nodeAndUserName map[node]string, args []string,
	passToKey, clusterName string) (node, map[node]string) {
	log.Println("Need to initiate swarm leader")
	var alias string
	alias = args[0]
	node := findNodeByAliasFromNodesYml(alias, nodesFromYml)
	host := node.Host
	config := findSSHKeysAndInitConnection(clusterName, nodeAndUserName[node], passToKey)
	configUfwToWorkInSwarmMode(host, config)
	log.Println("Starting swarm initialization...")
	sudoExecSSHCommand(host, "ufw allow 2377/tcp", config)
	reloadUfwAndDocker(host, config)
	sudoExecSSHCommand(host, "docker swarm init --advertise-addr "+host, config)
	delete(nodeAndUserName, node)
	node.SwarmMode = leader
	log.Println("Swarm initiated! Leader node is " + alias)
	return node, nodeAndUserName
}

func findNodeByAliasFromNodesYml(alias string, nodesFromYml []node) node {
	var leaderNode node
	for _, node := range nodesFromYml {
		if node.Alias == alias {
			leaderNode = node
		}
	}
	if leaderNode == (node{}) {
		log.Println("Can't find host by given alias in nodes.yml, choose it interactive")
		alias := numberHostsFromNodesFile(nodesFromYml)
		return findNodeByAliasFromNodesYml(alias, nodesFromYml)
	}
	return leaderNode
}

func getHostsFromNodesGroupingBySwarmModeValue(nodes []node) (node, [] node, [] node) {
	var clusterLeaderHost node
	var clusterManagerHosts []node
	var clusterWorkersHost []node
	for _, node := range nodes {
		if len(node.DockerVersion) == 0 {
			log.Fatal("Need to install docker on all nodes from nodes.yml, please exec `swarmgo docker`")
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
	return clusterLeaderHost, clusterManagerHosts, clusterManagerHosts
}

func configUfwToWorkInSwarmMode(host string, config *ssh.ClientConfig) {
	logWithPrefix(host, "Configuring ufw to work with swarm...")
	sudoExecSSHCommand(host, "ufw allow 22/tcp", config)
	sudoExecSSHCommand(host, "ufw allow 2376/tcp", config)
	sudoExecSSHCommand(host, "ufw allow 7946/tcp", config)
	sudoExecSSHCommand(host, "ufw allow 7946/udp", config)
	sudoExecSSHCommand(host, "ufw allow 4789/udp", config)
	logWithPrefix(host, "Ufw configured")
}

func joinToSwarm(node node, leaderHost, userName, passToKey, clusterName string) {
	host := node.Host
	config := findSSHKeysAndInitConnection(clusterName, userName, passToKey)
	configUfwToWorkInSwarmMode(host, config)
	var token string
	if mode {
		sudoExecSSHCommand(host, "ufw allow 2377/tcp", config)
		token = getToken("manager", leaderHost, config)
		node.SwarmMode = manager
	} else {
		token = getToken("worker", leaderHost, config)
		node.SwarmMode = worker
	}
	reloadUfwAndDocker(host, config)
	sudoExecSSHCommand(host, token, config)
	logWithPrefix(node.Host, node.Alias+" successfully joined swarm to swarm!")
	swarmChan <- node
}

func getSwarmLeaderNodeAndClusterFile() (*entry, *clusterFile) {
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
	return firstEntry, clusterFile
}

func init() {
	rootCmd.AddCommand(swarmCmd)
	swarmCmd.Flags().BoolVarP(&mode, "manager", "m", false, "Swarm mode: m means `join-manager")
}
