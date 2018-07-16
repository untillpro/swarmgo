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
		log.Println("Starting docker installation...")
		version := findDockerVersionFromClusterfile()
		nodesFileEntry := readFileIfExists(nodes, "Need to add some nodes first")
		hosts := takeHostsFromArgsOrChooseFromNodesFile(nodesFileEntry, args)
		fmt.Println("input password for public key")
		passToKey := waitUserInput()
		hostAndUserName := make(map[string]string)
		for _, host := range hosts {
			var userName string
			fmt.Println("input user name for host " + host)
			for len(userName) == 0 {
				fmt.Print("User name can't be empty!")
				userName = waitUserInput()
			}
			hostAndUserName[host] = userName
		}
		for _, value := range hosts {
			go func() {
				userName := hostAndUserName[value]
				config := findSshKeysAndInitConnection(userName, passToKey)
				installDocker(value, version, config)
			}()
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
	//TODO don't forget auto-check
	logWithPrefix(host, "Add Docker’s official GPG key")
	execSshCommand(host, "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -", config)
	logWithPrefix(host, "Verify key")
	logWithPrefix(host, sudoExecSshCommand(host, "apt-key fingerprint 0EBFCD88", config))
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

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// dockerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
}
