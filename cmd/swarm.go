/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cmd

import (
	"github.com/spf13/cobra"
	"log"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"strings"
	"fmt"
	"gopkg.in/yaml.v2"
	"path/filepath"
)

var mode bool
var swarmChan = make(chan Node)

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
		nodeHostAndNode := make(map[string]Node)
		for _, value := range nodesFromYml {
			nodeHostAndNode[value.Host] = value
		}
		if len(nodesFromYml) == 0 {
			log.Fatal("Can't find nodes from nodes.yml. Add some nodes first!")
		}
		clusterLeaderNode, clusterManagerNodes, clusterWorkerNodes := getHostsFromNodesGroupingBySwarmModeValue(nodesFromYml)
		if clusterLeaderNode == (Node{}) {
			if !mode {
				log.Fatal("Need to specify -manager to init swarm!")
			}
			if len(args) == 0 {
				log.Fatal("Need to pass at least one alias to init swarm!")
			}
		}
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		nodeAndUserName := make(map[Node]string)
		for _, node := range nodesFromYml {
			if node == clusterLeaderNode || containsNode(clusterManagerNodes, node) ||
				containsNode(clusterWorkerNodes, node) {
				log.Println(node.Alias + " already in swarm!")
				continue
			}
			var userName string
			if mode {
				if ok := contains(args, node.Alias); !ok {
					continue
				}
			}
			fmt.Println("input user name for host " + node.Host)
			for len(userName) == 0 {
				fmt.Println("User name can't be empty!")
				userName = waitUserInput()
			}
			nodeAndUserName[node] = userName
		}
		if len(nodeAndUserName) == 0 {
			log.Fatal("All nodes already in swarm")
		}
		var node Node
		if clusterLeaderNode == (Node{}) {
			node, nodeAndUserName = initSwarm(nodesFromYml, nodeAndUserName, args, passToKey, clusterFile.ClusterName)
			nodeHostAndNode[node.Host] = node
		}
		for key, value := range nodeAndUserName {
			go func(node Node, userName, passToKey string) {
				joinToSwarm(node, clusterLeaderNode.Host, userName, passToKey, clusterFile.ClusterName)
			}(key, value, passToKey)
		}
		for  key := range nodeAndUserName {
			nodeHostAndNode[key.Host] = <-swarmChan
		}
		close(swarmChan)
		nodes := make([]Node, len(nodeHostAndNode))
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
	output := sudoExecSshCommand(host, "docker swarm join-token "+mode, config)
	return strings.Trim(substringAfterIncludeValue(output, "docker swarm join"), "\n ")
}

func reloadUfwAndDocker(host string, config *ssh.ClientConfig) {
	log.Println("Restarting ufw and docker...")
	sudoExecSshCommand(host, "ufw reload", config)
	sudoExecSshCommand(host, "systemctl restart docker", config)
	log.Println("Ufw and docker restarted!")
}

func initSwarm(nodesFromYml []Node, nodeAndUserName map[Node]string, args []string,
	passToKey, clusterName string) (Node, map[Node]string) {
	log.Println("Need to initiate swarm leader")
	var alias string
	alias = args[0]
	node := findNodeByAliasFromNodesYml(alias, nodesFromYml)
	host := node.Host
	config := findSshKeysAndInitConnection(clusterName, nodeAndUserName[node], passToKey)
	configUfwToWorkInSwarmMode(host, config)
	log.Println("Starting swarm initialization...")
	sudoExecSshCommand(host, "ufw allow 2377/tcp", config)
	reloadUfwAndDocker(host, config)
	sudoExecSshCommand(host, "docker swarm init --advertise-addr "+host+":2377", config)
	delete(nodeAndUserName, node)
	node.SwarmMode = leader
	log.Println("Swarm initiated! Leader node is " + alias)
	return node, nodeAndUserName
}

func findNodeByAliasFromNodesYml(alias string, nodesFromYml []Node) Node {
	var leaderNode Node
	for _, node := range nodesFromYml {
		if node.Alias == alias {
			leaderNode = node
		}
	}
	if leaderNode == (Node{}) {
		log.Println("Can't find host by given alias in nodes.yml, choose it interactive")
		alias := numberHostsFromNodesFile(nodesFromYml)
		return findNodeByAliasFromNodesYml(alias, nodesFromYml)
	}
	return leaderNode
}

func getHostsFromNodesGroupingBySwarmModeValue(nodes []Node) (Node, [] Node, [] Node) {
	var clusterLeaderHost Node
	var clusterManagerHosts []Node
	var clusterWorkersHost []Node
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
	sudoExecSshCommand(host, "ufw allow 22/tcp", config)
	sudoExecSshCommand(host, "ufw allow 2376/tcp", config)
	sudoExecSshCommand(host, "ufw allow 7946/tcp", config)
	sudoExecSshCommand(host, "ufw allow 7946/udp", config)
	sudoExecSshCommand(host, "ufw allow 4789/udp", config)
	logWithPrefix(host, "Ufw configured")
}

func joinToSwarm(node Node, leaderHost, userName, passToKey, clusterName string) {
	host := node.Host
	config := findSshKeysAndInitConnection(clusterName, userName, passToKey)
	configUfwToWorkInSwarmMode(host, config)
	var token string
	if mode {
		sudoExecSshCommand(host, "ufw allow 2377/tcp", config)
		reloadUfwAndDocker(host, config)
		token = getToken("manager", leaderHost, config)
		node.SwarmMode = manager
	} else {
		reloadUfwAndDocker(host, config)
		token = getToken("worker", leaderHost, config)
		node.SwarmMode = worker
	}
	sudoExecSshCommand(host, token, config)
	logWithPrefix(node.Host, node.Alias + " successfully joined swarm to swarm!")
	swarmChan <- node
}

func init() {
	rootCmd.AddCommand(swarmCmd)
	swarmCmd.Flags().BoolVarP(&mode, "manager", "m", false, "Swarm mode: m means `join-manager")
}
