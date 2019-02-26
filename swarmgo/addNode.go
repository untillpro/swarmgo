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
	"log"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	yaml "gopkg.in/yaml.v2"
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

// addNodeCmd represents the addNode command
var addNodeCmd = &cobra.Command{
	Use:   "add",
	Short: "add <Alias1>=<1IP> <Alias2>=<IP2>",
	Long:  `Add node with specified alias and current IP to cluster and config access with keys`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if logs {
			f := redirectLogs()
			defer func() {
				if err := f.Close(); err != nil {
					log.Println("Error closing the file: ", err.Error())
				}
			}()
		}
		readFileIfExists(swarmgoConfigFileName, "Need to create swarmgo-config.yml first!")
		clusterFile := unmarshalClusterYml()
		debug("clusterFile", clusterFile)
		rootUserName := clusterFile.RootUserName
		debug("rootUserName", rootUserName)
		if strings.Trim(rootUserName, " \n") == "" {
			rootUserName = "root"
		}

		debug("ClusterName", clusterFile.ClusterName)
		debug("rootUserName", rootUserName)

		publicKeyFile, privateKeyFile := findSSHKeys(clusterFile)

		filesExist := FileExists(publicKeyFile) && FileExists(privateKeyFile)

		if !filesExist {
			fmt.Println("The following keys will be generated")
		}
		fmt.Println("Public Key:", publicKeyFile)
		fmt.Println("Private Key:", privateKeyFile)

		passToKey := readKeyPassword()
		if !filesExist {
			bitSize := 4096
			err := generateKeysAndWriteToFile(bitSize, privateKeyFile, publicKeyFile, passToKey)
			CheckErr(err)
		} else {
			log.Println("Keys already exist")
		}
		log.Println("Checking if node already configured to use keys")
		nodesFromYaml := getNodesFromYml(getCurrentDir())
		hosts := make([]string, len(nodesFromYaml))
		for index, node := range nodesFromYaml {
			hosts[index] = node.Alias + "=" + node.Host
		}
		i := 0
		for _, arg := range args {
			if contains(hosts, arg) {
				log.Println(arg + " node already configured to use keys!")
			} else {
				args[i] = arg
				i++
			}
		}
		args = args[:i]
		if len(args) == 0 {
			log.Fatal("All passed hosts already configured to use keys")
		}
		users := make([]user, len(args))
		for index, arg := range args {
			var user user
			userAndAlias := strings.Split(arg, "=")
			user.alias = userAndAlias[0]
			user.host = userAndAlias[1]
			user.rootUserName = rootUserName
			user.userName = clusterFile.ClusterUserName
			debug("Cluster user name", user.userName)
			user.passToRoot = readPasswordPrompt("Password for " + user.rootUserName + "@" + user.host)
			users[index] = user
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
			log.Println(errMsg)
		}
		close(nodesChannel)
		marshaledNode, err := yaml.Marshal(&nodesFromYaml)
		CheckErr(err)
		nodesFile := filepath.Join(getCurrentDir(), nodesFileName)
		err = ioutil.WriteFile(nodesFile, marshaledNode, 0600)
		CheckErr(err)
	},
}

func configHostToUseKeys(user user, publicKeyFile, privateKeyFile, passToKey string) error {
	host := user.host
	userName := user.userName
	rootUserName := user.rootUserName
	logWithPrefix(host, "Connecting to "+user.rootUserName+"@"+host)
	sshConfig := &ssh.ClientConfig{
		User:            rootUserName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(user.passToRoot)},
	}
	_, err := sudoExecSSHCommandWithoutPanic(host, "adduser --disabled-password --gecos \"\" "+userName, sshConfig)
	if err != nil {
		return err
	}
	pass := generateRandomString(32)
	logWithPrefix(host, "New user "+userName+" added")
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
	logWithPrefix(host, "Sudo permissions given to "+userName)
	sshConfig = &ssh.ClientConfig{
		User:            userName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
	}
	logWithPrefix(host, "Relogin from root to "+userName)
	_, err = sudoExecSSHCommandWithoutPanic(host, "passwd -l "+user.rootUserName, sshConfig)
	if err != nil {
		return err
	}
	logWithPrefix(host, "Root password disabled")
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
	logWithPrefix(host, "Host public key added to remote server")
	_, err = sudoExecSSHCommandWithoutPanic(host, "sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/g' /etc/ssh/sshd_config",
		sshConfig)
	if err != nil {
		return err
	}
	logWithPrefix(host, host+" password auth disabled")
	_, err = sudoExecSSHCommandWithoutPanic(host, "sudo service ssh restart", sshConfig)
	if err != nil {
		return err
	}
	logWithPrefix(host, "SSH reloaded")
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
	logWithPrefix(host, "Firewall reloaded to work with OpenSSH")
	return nil
}
