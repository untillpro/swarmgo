/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

const nodesFileName = "nodes.yml"

type user struct {
	host, alias, userName, passToRoot, rootUserName string
}
type node struct {
	Host, Alias, DockerVersion string
	SwarmMode                  string
	Traefik                    bool
}

func add(cmd *cobra.Command, args []string) {
	initCommand("add")
	defer finitCommand()

	// *************************************************
	gc.Doing("Reading config")

	readWorkingFileIfExists(swarmgoConfigFileName, "Config file not found, to create it run `swarmgo init`")
	clusterFile := unmarshalClusterYml()
	rootUserName := clusterFile.RootUserName
	if strings.Trim(rootUserName, " \n") == "" {
		rootUserName = "root"
	}

	gc.Verbose("clusterFile", clusterFile)
	gc.Verbose("ClusterName", clusterFile.ClusterName)
	gc.Verbose("RootUserName", rootUserName)

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

	gc.Verbose("nodesToAdd", nodesToAdd)
	gc.ExitIfFalse(len(nodesToAdd) > 0, "Nothing to add")

	// *************************************************
	gc.Doing("Checking keys")

	publicKeyFile, privateKeyFile := findSSHKeys(clusterFile)
	gc.Info("Public Key location:", publicKeyFile)
	gc.Info("Private Key location:", privateKeyFile)

	filesExist := FileExists(publicKeyFile) && FileExists(privateKeyFile)

	passToKey := readKeyPassword()
	if !filesExist {
		gc.Doing("Generating keys")
		bitSize := 4096
		err := generateKeysAndWriteToFile(bitSize, privateKeyFile, publicKeyFile, passToKey)
		gc.ExitIfError(err)
	}

	var users []user
	for name, IP := range nodesToAdd {
		var user user
		user.alias = name
		user.host = IP
		user.rootUserName = rootUserName
		user.userName = clusterFile.ClusterUserName
		user.passToRoot = readPasswordPrompt("Password for " + user.rootUserName + "@" + user.host)
		users = append(users, user)
	}

	nodesChannel := make(chan interface{})
	for _, value := range users {
		go func(user user) {
			//passToRoot to user and key from input
			err := configHostToUseKeys(user, publicKeyFile, privateKeyFile, passToKey)
			if err != nil {
				nodesChannel <- err
			} else {
				nodeFromFunc := node{
					Host:  user.host,
					Alias: user.alias,
				}
				nodesChannel <- nodeFromFunc
			}
		}(value)
	}
	errMsgs := make([]string, 0, len(args))
	for range args {
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
	err = ioutil.WriteFile(nodesFile, marshaledNode, 0600)
	gc.ExitIfError(err)
}

// addNodeCmd represents the addNode command
var addNodeCmd = &cobra.Command{
	Use:   "add",
	Short: "Configure SSH access to nodes and add nodes to nodes.yml",
	Long:  `Use add <node name1>=<IP1> <node name2>=<IP2> ...`,
	Args:  cobra.MinimumNArgs(1),
	Run:   add,
}

func configHostToUseKeys(user user, publicKeyFile, privateKeyFile, passToKey string) error {
	host := user.host
	userName := user.userName
	rootUserName := user.rootUserName

	// *************************************************
	sshConfig := &ssh.ClientConfig{
		User:            rootUserName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(user.passToRoot)},
	}

	doingWithPrefix(host, "Adding new user "+userName)
	_, err := sudoExecSSHCommandWithoutPanic(host, "adduser --disabled-password --gecos \"\" "+userName, sshConfig)
	if err != nil {
		return err
	}

	// *************************************************
	doingWithPrefix(host, "Giving sudo permissions to "+userName)
	pass := generateRandomString(32)
	_, err = sudoExecSSHCommandWithoutPanic(host, "echo \""+userName+":"+pass+"\" | sudo chpasswd", sshConfig)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "usermod -aG sudo "+userName, sshConfig)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "echo '"+userName+" ALL=(ALL:ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo", sshConfig)
	if err != nil {
		return err
	}

	// *************************************************
	doingWithPrefix(host, "Disabling the password for "+user.rootUserName)

	sshConfig = &ssh.ClientConfig{
		User:            userName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
	}

	_, err = sudoExecSSHCommandWithoutPanic(host, "passwd -l "+user.rootUserName, sshConfig)
	if err != nil {
		return err
	}

	// *************************************************
	doingWithPrefix(host, "Adding public key to host")
	_, err = execSSHCommandWithoutPanic(host, "mkdir -p ~/.ssh", sshConfig)
	if err != nil {
		return err
	}
	_, err = execSSHCommandWithoutPanic(host, "chmod 700 ~/.ssh", sshConfig)
	if err != nil {
		return err
	}
	_, err = execSSHCommandWithoutPanic(host, "touch ~/.ssh/authorized_keys", sshConfig)
	if err != nil {
		return err
	}
	_, err = execSSHCommandWithoutPanic(host, "chmod 600 ~/.ssh/authorized_keys", sshConfig)
	if err != nil {
		return err
	}
	//read public key
	pemBytes, err := ioutil.ReadFile(publicKeyFile)
	if err != nil {
		return err
	}
	_, err = execSSHCommandWithoutPanic(host, "echo \""+string(pemBytes)+"\" | tee ~/.ssh/authorized_keys", sshConfig)
	if err != nil {
		return err
	}

	// *************************************************
	doingWithPrefix(host, "Disabling password auth")
	_, err = sudoExecSSHCommandWithoutPanic(host, "sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/g' /etc/ssh/sshd_config",
		sshConfig)
	if err != nil {
		return err
	}

	// *************************************************
	doingWithPrefix(host, "Allowing ssh")
	_, err = sudoExecSSHCommandWithoutPanic(host, "sudo service ssh restart", sshConfig)
	if err != nil {
		return err
	}
	sshConfig = initSSHConnectionConfigWithPublicKeys(userName, privateKeyFile, passToKey)
	_, err = sudoExecSSHCommandWithoutPanic(host, "ufw allow OpenSSH", sshConfig)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "yes | sudo ufw enable", sshConfig)
	if err != nil {
		return err
	}
	_, err = sudoExecSSHCommandWithoutPanic(host, "ufw reload", sshConfig)
	if err != nil {
		return err
	}
	logWithPrefix(host, "Done")
	return nil
}
