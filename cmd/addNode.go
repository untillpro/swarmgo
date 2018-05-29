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
	"io/ioutil"
	"log"
	"os"
	"strings"
)

const (
	nodes    = "nodes"
	userName = "cluster"
)

var channel  = make(chan string)

// addNodeCmd represents the addNode command
var addNodeCmd = &cobra.Command{
	Use:   "add",
	Short: "add <arg1 arg2 arg3 ...> all args must be valid hosts!",
	Long:  `Add node with current IP to cluster and do some installations???`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		readClusterfileIfExists()
		if len(args) == 0 {
			log.Fatal("You must pass one or mode ip!")
		}
		publicKeyFile, privateKeyFile := findSshKeys()
		if !checkFileExistence(publicKeyFile) && !checkFileExistence(privateKeyFile) {
			bitSize := 4096
			err := generateKeysAndWriteToFile(bitSize, privateKeyFile, publicKeyFile)
			CheckErr(err)
		} else {
			log.Println("Keys already exist")
		}
		log.Println("Checking if node already configured to use keys")
		nodesFileName := appendChildToExecutablePath(nodes)
		nodesFile, err := os.OpenFile(nodesFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
		CheckErr(err)
		defer nodesFile.Close()
		fileText, err := ioutil.ReadAll(nodesFile)
		CheckErr(err)
		userAndHost := strings.Split(string(fileText), "\n")
		i := 0
		for _, j := range args {
			if contains(userAndHost, j) {
				log.Println(j + " node already configured to use keys!")
			} else {
				args[i] = j
				i++
			}
		}
		args = args[:i]
		if len(args) == 0 {
			log.Fatal("All passed hosts already configured to use keys")
		}
		log.Println("Connecting to remote servers root with password..")
		for _, k := range args {
			go func(host string) {
				configHostToUseKeys(userName, host, publicKeyFile, privateKeyFile)
				logWithPrefix(host, "Write host to nodes file")
				if _, err = nodesFile.WriteString(host + "\n"); err != nil {
					panic(err)
				}
			}(k)
		}
		//to wait all results
		for range args {
			res := <-channel
			log.Println(res)
		}
	},
}

func configHostToUseKeys(userName, host, publicKeyFile, privateKeyFile string) {
	logWithPrefix(host,"Host " + host)
	logWithPrefix(host,"Connecting to remote servers root with password..")
	sshConfig := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		//pass password from args?
		Auth: []ssh.AuthMethod{ssh.Password("pas")},
	}
	//add userName "cluster"
	execSshCommand(host, "adduser --disabled-password --gecos \"\" "+userName, sshConfig)
	logWithPrefix(host,"New user " + userName + " added")
	//generate random password for userName cluster
	pass := generateRandomString(32)
	execSshCommand(host, "echo \""+userName+":"+pass+"\" | sudo chpasswd", sshConfig)
	execSshCommand(host, "usermod -aG sudo "+userName, sshConfig)
	logWithPrefix(host,"Sudo permissions given to " + userName)
	sshConfig = &ssh.ClientConfig{
		User:            userName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		//pass password from args?
		Auth: []ssh.AuthMethod{ssh.Password(pass)},
	}
	logWithPrefix(host,"Relogin from root to " + userName)
	sudoExecSshCommand(host, "passwd -l root", sshConfig)
	logWithPrefix(host,"Root password disabled")
	execSshCommand(host, "mkdir ~/.ssh", sshConfig)
	execSshCommand(host, "chmod 700 ~/.ssh", sshConfig)
	execSshCommand(host, "touch ~/.ssh/authorized_keys", sshConfig)
	execSshCommand(host, "chmod 600 ~/.ssh/authorized_keys", sshConfig)
	//read public key
	pemBytes, err := ioutil.ReadFile(publicKeyFile)
	CheckErr(err)
	execSshCommand(host, "echo \""+string(pemBytes)+"\" | tee ~/.ssh/authorized_keys", sshConfig)
	logWithPrefix(host,"Host public key added to remote server")
	sudoExecSshCommand(host, "sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/g' /etc/ssh/sshd_config",
		sshConfig)
	logWithPrefix(host,host + " password auth disabled")
	sshConfig = initSshConnectionConfigWithPublicKeys(privateKeyFile)
	sudoExecSshCommand(host, "ufw allow OpenSSH", sshConfig)
	sudoExecSshCommand(host, "yes | sudo ufw enable", sshConfig)
	sudoExecSshCommand(host, "ufw reload", sshConfig)
	logWithPrefix(host,"Firewall reloaded to work with OpenSSH")
	channel <- host + " done!"
}

func init() {
	nodeCmd.AddCommand(addNodeCmd)
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addNodeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addNodeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
