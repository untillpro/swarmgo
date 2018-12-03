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

const nodesFileName = "nodes.yml"

type user struct {
	host, alias, userName, passToRoot, passToUser string
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
			defer f.Close()
		}
		readFileIfExists(swarmgoConfigFileName, "Need to use swarmgo init first!")
		clusterFile := unmarshalClusterYml()
		publicKeyFile, privateKeyFile := findSSHKeys(clusterFile.ClusterName)
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		if !checkFileExistence(publicKeyFile) && !checkFileExistence(privateKeyFile) {
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
			fmt.Println("input user name for host " + user.host)
			for len(user.userName) == 0 {
				fmt.Println("User name can't be empty!")
				user.userName = waitUserInput()
			}
			fmt.Println("input password for root user of " + user.host)
			user.passToRoot = waitUserInput()
			fmt.Println("input password for new user of " + user.host)
			user.passToUser = waitUserInput()
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
		for range args {
			nodeFromChannel := <-nodesChannel
			switch nodeType := nodeFromChannel.(type) {
			case node:
				nodesFromYaml = append(nodesFromYaml, nodeType)
			case error:
				log.Println(nodeType.Error())
			}
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
	logWithPrefix(host, "Host "+host)
	logWithPrefix(host, "Connecting to remote servers root with password..")
	sshConfig := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(user.passToRoot)},
	}
	_, err := execSSHCommandWithoutPanic(host, "adduser --disabled-password --gecos \"\" "+userName, sshConfig)
	if err != nil {
		return err
	}
	logWithPrefix(host, "New user "+userName+" added")
	_, err = execSSHCommandWithoutPanic(host, "echo \""+userName+":"+user.passToUser+"\" | sudo chpasswd", sshConfig)
	if err != nil {
		return err
	}
	_, err = execSSHCommandWithoutPanic(host, "usermod -aG sudo "+userName, sshConfig)
	if err != nil {
		return err
	}
	_, err = execSSHCommandWithoutPanic(host, "echo '"+userName+" ALL=(ALL:ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo", sshConfig)
	if err != nil {
		return err
	}
	logWithPrefix(host, "Sudo permissions given to "+userName)
	sshConfig = &ssh.ClientConfig{
		User:            userName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.Password(user.passToUser)},
	}
	logWithPrefix(host, "Relogin from root to "+userName)
	_, err = sudoExecSSHCommandWithoutPanic(host, "passwd -l root", sshConfig)
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

func init() {
	rootCmd.AddCommand(addNodeCmd)
}
