/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cli

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"gopkg.in/yaml.v2"
)

const nodesFileName = "nodes.yml"

var skipSSHConfiguration bool = false
var argRootPassword string = ""

type user struct {
	host, alias, userName, rootUserName string
}
type node struct {
	Host, Alias, DockerVersion string
	SwarmMode                  string
	Uname                      string
	Traefik                    bool
}

// AddNodes adds nodes to cluster configuration
func AddNodes(nodesToAdd map[string]string, rootPassword string, skipSSH bool) {
	gc.Info("Adding nodes", nodesToAdd)
	gc.ExitIfFalse(len(nodesToAdd) > 0, "Nothing to add")

	// *************************************************
	nodesFromYaml := getNodesFromYml(getWorkingDir())
	clusterFile := unmarshalClusterYml()
	rootUserName := clusterFile.RootUserName
	if strings.Trim(rootUserName, " \n") == "" {
		rootUserName = "root"
	}

	gc.Verbose("clusterFile", clusterFile)
	gc.Verbose("ClusterName", clusterFile.ClusterName)
	gc.Verbose("RootUserName", rootUserName)
	var users []user
	for name, IP := range nodesToAdd {
		var user user
		user.alias = name
		user.host = IP
		user.rootUserName = rootUserName
		user.userName = clusterFile.ClusterUserName
		users = append(users, user)
	}

	publicKeyFile, privateKeyFile := findSSHKeys(clusterFile)

	if !skipSSH {
		for _, value := range users {
			err := configHostToUseKeys(value, publicKeyFile, rootPassword)
			gc.ExitIfError(err, "Unable to add user for node: "+value.host)
		}
	}

	nodesChannel := make(chan interface{})
	for _, value := range users {
		go func(user user) {
			client := getSSHClientInstance(user.userName, privateKeyFile)
			uname, err := client.Exec(user.host, "uname -a")
			if err == nil && !skipSSH {
				err = configureFirewall(user.host, client)
			}
			if err != nil {
				nodesChannel <- err
			} else {
				nodeFromFunc := node{
					Host:  user.host,
					Alias: user.alias,
					Uname: uname,
				}
				nodesChannel <- nodeFromFunc
			}
		}(value)
	}
	errMsgs := make([]string, 0, len(nodesToAdd))
	for range nodesToAdd {
		nodeFromChannel := <-nodesChannel
		switch nodeFromChannel.(type) {
		case node:
			nodesFromYaml = append(nodesFromYaml, nodeFromChannel.(node))
		case error:
			errMsgs = append(errMsgs, nodeFromChannel.(error).Error())
		}
	}
	for _, errMsg := range errMsgs {
		gc.Info(errMsg)
	}
	close(nodesChannel)
	marshaledNode, err := yaml.Marshal(&nodesFromYaml)
	gc.ExitIfError(err)
	nodesFile := filepath.Join(getWorkingDir(), nodesFileName)
	gc.ExitIfError(ioutil.WriteFile(nodesFile, marshaledNode, 0600))
	gc.ExitIfFalse(len(errMsgs) == 0, "Failed to add some node(s)")
	gc.Info("All nodes added")
}

func add(cmd *cobra.Command, args []string) {
	initCommand("add")
	defer finitCommand()

	checkSSHAgent()

	// *************************************************
	gc.Doing("Reading configuration")

	readWorkingFileIfExists(swarmgoConfigFileName, "Config file not found, to create it run `swarmgo init`")

	nodesFromYaml := getNodesFromYml(getWorkingDir())

	// *************************************************
	gc.Doing("Getting existing nodeNames and nodeIPs")

	nodeNames := make(map[string]string)
	nodeIPs := make(map[string]string)
	for _, node := range nodesFromYaml {
		nodeNames[node.Alias] = node.Host
		nodeIPs[node.Host] = node.Alias
	}

	// *************************************************
	gc.Doing("Calculating which nodes to add")

	nodesToAdd := make(map[string]string)

	for _, arg := range args {
		userAndAlias := strings.Split(arg, "=")
		gc.ExitIfFalse(len(userAndAlias) == 2, "Wrong argument:`", arg, " ` must be <node name>=<node IP>")
		userAndAlias[0] = strings.TrimSpace(userAndAlias[0])
		userAndAlias[1] = strings.TrimSpace(userAndAlias[1])

		gc.ExitIfFalse(len(userAndAlias[0]) > 0, "Wrong argument: ", arg)
		gc.ExitIfFalse(len(userAndAlias[1]) > 0, "Wrong argument: ", arg)

		nodeName := userAndAlias[0]
		nodeIP := userAndAlias[1]

		if value, ex := nodeNames[nodeName]; ex {
			gc.Info("Name already configured:", nodeName, value)
			continue
		}

		if value, ex := nodeIPs[nodeIP]; ex {
			gc.Info("IP already configured:", nodeIP, value)
			continue
		}
		nodesToAdd[userAndAlias[0]] = userAndAlias[1]
	}

	AddNodes(nodesToAdd, argRootPassword, skipSSHConfiguration)
}

var addNodeCmd = &cobra.Command{
	Use:   "add",
	Short: "Configure SSH access to nodes and add nodes to nodes.yml. Use -s option to skip SSH configuration",
	Long:  `Use add <node name1>=<IP1> <node name2>=<IP2> ...`,
	Args:  cobra.MinimumNArgs(1),
	Run:   add,
}

func configHostToUseKeys(user user, publicKeyFile string, rootPass string) error {

	host := user.host
	userName := user.userName
	rootUserName := user.rootUserName
	logWithPrefix(host, "Configuring cluster user...")

	scriptPath := filepath.Join(getSourcesDir(), "/scripts/adduser.sh")
	scriptBytes, err := ioutil.ReadFile(scriptPath)
	gc.ExitIfError(err, "Unable to read script from "+scriptPath)

	pemBytes, err := ioutil.ReadFile(publicKeyFile)
	gc.ExitIfError(err, "Unable to read public key from "+publicKeyFile)

	setupCmd := "echo '" + string(scriptBytes) + "' > ~/setup.sh && chmod 700 ~/setup.sh && ./setup.sh " + userName + " " + generateRandomString(32) + " \"" + string(pemBytes) + "\" && rm ~/setup.sh"

	client := getSSHClientInstance(rootUserName, "")
	client.Password = rootPass
	client.HideStdout = len(rootPass) > 0 // When password specified, no input expected from user
	_, err = client.Exec(host, setupCmd)
	if err != nil {
		return err
	}

	logWithPrefix(host, "Node added")
	return nil
}

func configureFirewall(host string, client *SSHClient) error {
	commands := []SSHCommand{
		SSHCommand{
			cmd:   "sudo ufw allow OpenSSH",
			title: "Allowing OpenSSH in firewall",
		},
		SSHCommand{
			cmd:   "sudo yes | sudo ufw enable",
			title: "Enabling firewall",
		},
		SSHCommand{
			cmd:   "sudo ufw reload",
			title: "Reloading firewall",
		},
	}
	return sshKeyAuthCmds(host, client, commands)
}
