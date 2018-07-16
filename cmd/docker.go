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
	"fmt"
)

const docker = "docker-ce"

// dockerCmd represents the docker command
var dockerCmd = &cobra.Command{
	Use:   "docker <arg1 arg2...> or not",
	Short: "Install docker",
	Long:  `Downloads and installs docker specific version. Version takes from Clusterfile`,
	Run: func(cmd *cobra.Command, args []string) {
		dockerVersion := unmarshalClusterYml().Docker
		nodesFileEntry := readFileIfExists(nodes, "Need to add some nodes first")
		hosts := getNodesFromFileEntry(nodesFileEntry)
		fmt.Println("input password for public key")
		passToKey := waitUserInput()
		hostAndUserName := make(map[string]string)
		for _, host := range hosts {
			var userName string
			fmt.Println("input user name for host " + host)
			for len(userName) == 0 {
				fmt.Println("User name can't be empty!")
				userName = waitUserInput()
			}
			hostAndUserName[host] = userName
		}
		for key, value := range hostAndUserName {
			go func(host string, userName string) {
				config := findSshKeysAndInitConnection(userName, passToKey)
				installDocker(host, dockerVersion, config)
			}(key, value)
		}
		for range hosts {
			res := <-channel
			log.Println(res)
		}
	},
}

func installDocker(host, version string, config *ssh.ClientConfig) {
	if checkDockerInstallation(host, version, config) {
		logWithPrefix(host, "Docker version "+version+" already installed!")
		channel <- "already installed"
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
		channel <- "Docker succesfully installed!"
	} else {
		channel <- "Can't install docker"
	}
}

func init() {
	rootCmd.AddCommand(dockerCmd)
}
