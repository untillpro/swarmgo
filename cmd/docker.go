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
	"golang.org/x/crypto/ssh"
	"fmt"
	"gopkg.in/yaml.v2"
	"log"
	"io/ioutil"
	"path/filepath"
)

var channelForNodes = make(chan Node)

const docker = "docker-ce"

// dockerCmd represents the docker command
var dockerCmd = &cobra.Command{
	Use:   "docker <arg1 arg2...> or not",
	Short: "Install docker",
	Long:  `Downloads and installs docker specific version. Version takes from Clusterfile`,
	Run: func(cmd *cobra.Command, args []string) {
		dockerVersion := unmarshalClusterYml().Docker
		nodesFromYaml := getNodesFromYml(getCurrentDir())
		if len(nodesFromYaml) == 0 {
			log.Fatal("Can't find nodes from nodes.yml. Add some nodes first!")
		}
		log.Println("Checking docker version " + dockerVersion + " already installed")
		alreadyInstalled := make([]Node, 0, len(nodesFromYaml))
		notInstalled := make([]Node, 0, len(nodesFromYaml))
		for _, node := range nodesFromYaml {
			if dockerVersion == node.DockerVersion {
				log.Println("Docker already installer on " + node.Host)
				alreadyInstalled = append(alreadyInstalled, node)
			} else {
				notInstalled = append(notInstalled, node)
			}
		}
		if len(notInstalled) == 0 {
			log.Fatal("Docker version " + dockerVersion + " already installed on all nodesFileName")
		}
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		nodeAndUserName := make(map[Node]string)
		for _, node := range notInstalled {
			var userName string
			fmt.Println("input user name for host " + node.Host)
			for len(userName) == 0 {
				fmt.Println("User name can't be empty!")
				userName = waitUserInput()
			}
			nodeAndUserName[node] = userName
		}
		for key, value := range nodeAndUserName {
			go func(node Node, userName string) {
				config := findSshKeysAndInitConnection(userName, passToKey)
				installDocker(node, dockerVersion, config)
			}(key, value)
		}
		nodes := make([]Node, 0, len(args))
		for range nodeAndUserName {
			nodes = append(nodes,<-channelForNodes)
		}
		nodes = append(nodes, alreadyInstalled...)
		close(channelForNodes)
		marshaledNode, err := yaml.Marshal(&nodes)
		CheckErr(err)
		nodesFilePath := filepath.Join(getCurrentDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		CheckErr(err)
	},
}

func installDocker(node Node, version string, config *ssh.ClientConfig) {
	host := node.Host
	if checkDockerInstallation(host, version, config) {
		logWithPrefix(host, "Docker version "+version+" already installed!")
		node.DockerVersion = version
		channelForNodes <- node
		return
	}
	logWithPrefix(host, "Updating apt-get...")
	sudoExecSshCommand(host, "apt-get update", config)
	logWithPrefix(host, "Installing packages to allow apt to use a repository over HTTPS...")
	sudoExecSshCommand(host, "apt-get -y install apt-transport-https ca-certificates curl "+
		"software-properties-common", config)
	logWithPrefix(host, "Add Docker’s official GPG key")
	execSshCommand(host, "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -", config)
	logWithPrefix(host, "Adding repository")
	sudoExecSshCommand(host, "add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu "+
		"$(lsb_release -cs) stable\"", config)
	logWithPrefix(host, "Updating apt-get...")
	sudoExecSshCommand(host, "apt-get update", config)
	logWithPrefix(host, "Trying to install "+docker+" version "+version+"...")
	sudoExecSshCommand(host, "apt-get -y install "+docker+"="+version, config)
	logWithPrefix(host, "Checking installation...")
	if checkDockerInstallation(host, version, config) {
		logWithPrefix(host, "Docker successfully installed")
		node.DockerVersion = version
		channelForNodes <- node
	} else {
		logWithPrefix(host, "Can't install docker")
		node.DockerVersion = ""
		channelForNodes <- node
	}
}

func init() {
	rootCmd.AddCommand(dockerCmd)
}
