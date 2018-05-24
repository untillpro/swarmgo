// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

const ( nodes = "nodes"; userName = "cluster" )

// addNodeCmd represents the addNode command
var addNodeCmd = &cobra.Command{
	Use:   "add",
	Short: "add [#ip]",
	Long:  `Add node with current IP to cluster and do some installations???`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		readClusterfileIfExists()
		host := ip
		if len(ip) == 0 {
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
		if contains(userAndHost, host) {
			log.Fatal("Node already configured to use keys!")
		}
		log.Println("Connecting to remote servers root with password..")
		configHostToUseKeys(userName, host, publicKeyFile, privateKeyFile)
		log.Println("Write host to nodes file")
		if _, err = nodesFile.WriteString(host + "\n"); err != nil {
			panic(err)
		}
	},
}

func configHostToUseKeys(userName, host, publicKeyFile, privateKeyFile string) {
	log.Println("Host " + host)
	log.Println("Connecting to remote servers root with password..")
	sshConfig := &ssh.ClientConfig{
		User:            "root",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		//pass password from args?
		Auth: []ssh.AuthMethod{ssh.Password("pas")},
	}
	//add userName "cluster"
	execSshCommand(host, "adduser --disabled-password --gecos \"\" "+userName, sshConfig)
	log.Println("New user " + userName + " added")
	//generate random password for userName cluster
	pass := generateRandomString(32)
	execSshCommand(host, "echo \""+userName+":"+pass+"\" | sudo chpasswd", sshConfig)
	execSshCommand(host, "usermod -aG sudo "+userName, sshConfig)
	log.Println("Sudo permissions given to " + userName)
	sshConfig = &ssh.ClientConfig{
		User:            userName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		//pass password from args?
		Auth: []ssh.AuthMethod{ssh.Password(pass)},
	}
	log.Println("Relogin from root to " + userName)
	sudoExecSshCommand(host, "passwd -l root", sshConfig)
	log.Println("Root password disabled")
	execSshCommand(host, "mkdir ~/.ssh", sshConfig)
	execSshCommand(host, "chmod 700 ~/.ssh", sshConfig)
	execSshCommand(host, "touch ~/.ssh/authorized_keys", sshConfig)
	execSshCommand(host, "chmod 600 ~/.ssh/authorized_keys", sshConfig)
	//read public key
	pemBytes, err := ioutil.ReadFile(publicKeyFile)
	CheckErr(err)
	execSshCommand(host, "echo \""+string(pemBytes)+"\" | tee ~/.ssh/authorized_keys", sshConfig)
	log.Println("Host public key added to remote server")
	sudoExecSshCommand(host, "sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/g' /etc/ssh/sshd_config",
		sshConfig)
	log.Println(host + " password auth disabled")
	sshConfig = initSshConnectionConfigWithPublicKeys(privateKeyFile)
	sudoExecSshCommand(host, "ufw allow OpenSSH", sshConfig)
	sudoExecSshCommand(host, "yes | sudo ufw enable", sshConfig)
	sudoExecSshCommand(host, "ufw reload", sshConfig)
	log.Println("Firewall reloaded to work with OpenSSH")
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
