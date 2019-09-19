/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

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

var forceUpgradeDocker bool

// dockerCmd represents the docker command
var dockerCmd = &cobra.Command{
	Use:   "docker <arg1 arg2...> or not",
	Short: "Install docker. Use -u flag to upgrade",
	Long:  `Downloads and installs latest version of docker`,
	Run: loggedCmd(func(args []string) {

		clusterFile := unmarshalClusterYml()
		nodesFromYaml := getNodesFromYml(getWorkingDir())
		gc.ExitIfFalse(len(nodesFromYaml) > 0, "Can't find nodes from nodes.yml. Add some nodes first!")

		aliasesAndNodes := make(map[string]node)
		for _, node := range nodesFromYaml {
			aliasesAndNodes[node.Alias] = node
		}
		nodesForDocker := make([]node, 0, len(nodesFromYaml))
		if len(args) != 0 {
			for _, arg := range args {
				val, ok := aliasesAndNodes[arg]
				gc.ExitIfFalse(ok, "missing in nodes.yml")
				nodesForDocker = append(nodesForDocker, val)
			}
		} else {
			nodesForDocker = nodesFromYaml
		}
		passToKey := readKeyPassword()
		var channelForNodes = make(chan nodeAndError)
		for _, currentNode := range nodesForDocker {
			go func(node node) {
				config := findSSHKeysAndInitConnection(passToKey, clusterFile)
				nodeFromGoroutine, err := installDocker(node, config)
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
		gc.ExitIfError(err)
		nodesFilePath := filepath.Join(getWorkingDir(), nodesFileName)
		gc.ExitIfError(ioutil.WriteFile(nodesFilePath, marshaledNode, 0600))
		gc.ExitIfFalse(len(errMsgs) == 0, "Failed to install docker on some node(s)")
	}),
}

func installDocker(node node, config *ssh.ClientConfig) (node, error) {
	host := node.Host

	//check that already installed, use "force" flag to force update
	version, err := getDockerVersion(host, config)
	if err == nil && version != "" {
		if !forceUpgradeDocker {
			logWithPrefix(host, fmt.Sprintf("Docker version [%s] already installed! Use -u flag to update docker to the latest version", version))
			node.DockerVersion = version
			return node, err
		} else {
			logWithPrefix(host, fmt.Sprintf("Docker version [%s] already installed and will be upgraded to the latest version", version))
		}
	} else {
		logWithPrefix(host, "Couldn't find docker, installing latest version")
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
	logWithPrefix(host, "Trying to install the latest version of "+docker)
	_, err = sudoExecSSHCommandWithoutPanic(host, "apt-get -y install "+docker, config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}
	logWithPrefix(host, "Checking installation...")

	version, err = getDockerVersion(host, config)
	if err != nil {
		node.DockerVersion = ""
		return node, err
	}

	logWithPrefix(host, "Docker successfully installed")
	node.DockerVersion = version

	return node, nil
}

func getDockerVersion(host string, config *ssh.ClientConfig) (string, error) {
	stdout, err := sudoExecSSHCommandWithoutPanic(host, "docker -v", config)

	if err != nil {
		return "", err
	}

	version := ParseDockerVersion(stdout)

	if version == "" {
		return "", errors.New("Unable to retrieve docker version from output: " + stdout)
	}

	gc.Verbose("docker -v", stdout)
	gc.Verbose("version", version)

	return version, nil
}
