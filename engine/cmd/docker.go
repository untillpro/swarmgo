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

const docker = "docker-ce"

type nodeAndError struct {
	nodeWithPossibleError node
	err                   error
}

// dockerCmd represents the docker command
var dockerCmd = &cobra.Command{
	Use:   "docker <arg1 arg2...> or not",
	Short: "Install docker",
	Long:  `Downloads and installs docker specific version. Version takes from Clusterfile`,
	Run: func(cmd *cobra.Command, args []string) {
		if logs {
			f := redirectLogs()
			defer func() {
				if err := f.Close(); err != nil {
					log.Println("Error closing the file: ", err.Error())
				}
			}()
		}
		clusterFile := unmarshalClusterYml()
		dockerVersion := clusterFile.Docker
		nodesFromYaml := getNodesFromYml(getCurrentDir())
		if len(nodesFromYaml) == 0 {
			log.Fatal("Can't find nodes from nodes.yml. Add some nodes first!")
		}
		alreadyInstalled := make([]node, 0, len(nodesFromYaml))
		notInstalled := make([]node, 0, len(nodesFromYaml))
		aliasesAndNodes := make(map[string]node)
		for _, node := range nodesFromYaml {
			aliasesAndNodes[node.Alias] = node
		}
		nodesForDocker := make([]node, 0, len(nodesFromYaml))
		if len(args) != 0 {
			for _, arg := range args {
				val, ok := aliasesAndNodes[arg]
				if !ok {
					log.Fatal(val, "doesn't present in nodes.yml")
				}
				nodesForDocker = append(nodesForDocker, val)
			}
		} else {
			nodesForDocker = nodesFromYaml
		}
		for _, node := range nodesForDocker {
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
		var channelForNodes = make(chan nodeAndError)
		for _, currentNode := range notInstalled {
			go func(node node) {
				config := findSSHKeysAndInitConnection(clusterFile.ClusterName, clusterFile.ClusterUserName, passToKey)
				nodeFromGoroutine, err := installDocker(node, dockerVersion, config)
				nodeFromFunc := nodeAndError{
					nodeFromGoroutine,
					err,
				}
				channelForNodes <- nodeFromFunc
			}(currentNode)
		}
		errMsgs := make([]string, 0, len(args))
		for range notInstalled {
			nodeWithPossibleError := <-channelForNodes
			node := nodeWithPossibleError.nodeWithPossibleError
			err := nodeWithPossibleError.err
			if nodeWithPossibleError.err != nil {
				errMsgs = append(errMsgs, fmt.Sprintf("Host: %v, returns error: %v", node.Host,
					err.Error()))
			}
			aliasesAndNodes[node.Alias] = node
		}
		for _, errMsg := range errMsgs {
			log.Println(errMsg)
		}
		close(channelForNodes)
		nodes := make([]node, 0, len(aliasesAndNodes))
		for _, val := range aliasesAndNodes {
			nodes = append(nodes, val)
		}
		marshaledNode, err := yaml.Marshal(&nodes)
		CheckErr(err)
		nodesFilePath := filepath.Join(getCurrentDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFilePath, marshaledNode, 0600)
		CheckErr(err)
	},
}

func installDocker(node node, version string, config *ssh.ClientConfig) (node, error) {
	host := node.Host
	if checkDockerInstallation(host, version, config) {
		logWithPrefix(host, "Docker version "+version+" already installed!")
		node.DockerVersion = version
		return node, nil
	}
	logWithPrefix(host, "Updating apt-get...")
	_, err := sudoExecSSHCommandWithoutPanic(host, "apt-get update", config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	logWithPrefix(host, "Installing packages to allow apt to use a repository over HTTPS...")
	_, err = sudoExecSSHCommandWithoutPanic(host, "apt-get -y install apt-transport-https ca-certificates curl "+
		"software-properties-common", config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	logWithPrefix(host, "Add Docker’s official GPG key")
	_, err = execSSHCommandWithoutPanic(host, "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -", config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	logWithPrefix(host, "Adding repository")
	_, err = sudoExecSSHCommandWithoutPanic(host, "add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu "+
		"$(lsb_release -cs) stable\"", config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	logWithPrefix(host, "Updating apt-get...")
	_, err = sudoExecSSHCommandWithoutPanic(host, "apt-get update", config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	logWithPrefix(host, "Trying to install "+docker+" version "+version+"...")
	_, err = sudoExecSSHCommandWithoutPanic(host, "apt-get -y install "+docker+"="+version, config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	logWithPrefix(host, "Checking installation...")
	if checkDockerInstallation(host, version, config) {
		logWithPrefix(host, "Docker successfully installed")
		node.DockerVersion = version
		return node, nil
	} else {
		logWithPrefix(host, "Can't install docker")
		node.DockerVersion = ""
		return node, nil
	}
}

func checkDockerInstallation(host, version string, config *ssh.ClientConfig) bool {
	exit, _ := sudoExecSSHCommandWithoutPanic(host, "docker -v", config)
	trimmedVersion := strings.Split(version, "~")[0]
	return strings.Contains(exit, trimmedVersion)
}

func init() {
	rootCmd.AddCommand(dockerCmd)
}
