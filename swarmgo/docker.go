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
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
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
		initCommand("docker")
		defer finitCommand()
		clusterFile := unmarshalClusterYml()
		nodesFromYaml := getNodesFromYml(getCurrentDir())
		if len(nodesFromYaml) == 0 {
			gc.Fatal("Can't find nodes from nodes.yml. Add some nodes first!")
		}
		aliasesAndNodes := make(map[string]node)
		for _, node := range nodesFromYaml {
			aliasesAndNodes[node.Alias] = node
		}
		nodesForDocker := make([]node, 0, len(nodesFromYaml))
		if len(args) != 0 {
			for _, arg := range args {
				val, ok := aliasesAndNodes[arg]
				if !ok {
					gc.Fatal(val, "doesn't present in nodes.yml")
				}
				nodesForDocker = append(nodesForDocker, val)
			}
		} else {
			nodesForDocker = nodesFromYaml
		}
		passToKey := waitUserInput()
		var channelForNodes = make(chan nodeAndError)
		for _, currentNode := range nodesForDocker {
			go func(node node) {
				config := findSSHKeysAndInitConnection(passToKey, clusterFile)
				nodeFromGoroutine, err := installDocker(node, clusterFile.Docker, config)
				nodeFromFunc := nodeAndError{
					nodeFromGoroutine,
					err,
				}
				channelForNodes <- nodeFromFunc
			}(currentNode)
		}
		errMsgs := make([]string, 0, len(args))
		for range nodesForDocker {
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
			gc.Info(errMsg)
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

func installDocker(node node, dockerVersions map[string]map[string]string, config *ssh.ClientConfig) (node, error) {
	host := node.Host
	oSName, err := sudoExecSSHCommandWithoutPanic(host, "lsb_release -i", config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	oSVersion, err := sudoExecSSHCommandWithoutPanic(host, "lsb_release -r", config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	oSName = strings.Trim(substringAfter(oSName, "Distributor ID:"), " \t\n")
	oSVersion = strings.Trim(substringAfter(oSVersion, "Release:"), " \t\n")
	version := dockerVersions[oSName][oSVersion]
	if checkDockerInstallation(host, version, config) || version == node.DockerVersion {
		logWithPrefix(host, "Docker version "+version+" already installed!")
		node.DockerVersion = version
		return node, nil
	}
	logWithPrefix(host, "Updating apt-get...")
	_, err = sudoExecSSHCommandWithoutPanic(host, "apt-get update", config)
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
	}
	logWithPrefix(host, "Can't install docker")
	node.DockerVersion = ""
	return node, nil
}

func checkDockerInstallation(host, version string, config *ssh.ClientConfig) bool {
	exit, _ := sudoExecSSHCommandWithoutPanic(host, "docker -v", config)

	re := regexp.MustCompile("(?:.*:)?([^~]*)")
	submatch := re.FindStringSubmatch(version)
	var trimmedVersion string
	if len(submatch) > 1 {
		trimmedVersion = submatch[1]
	} else {
		trimmedVersion = ""
	}
	gc.Verbose("docker -v", exit)
	gc.Verbose("version", version)
	gc.Verbose("trimmedVersion", trimmedVersion)

	return strings.Contains(exit, trimmedVersion)
}
