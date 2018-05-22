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
	"github.com/mitchellh/go-homedir"
	"path/filepath"
	"log"
)

// addNodeCmd represents the addNode command
var addNodeCmd = &cobra.Command{
	Use:   "add",
	Short: "add <ip>",
	Long:  `Add node with current IP to cluster and do some installations???`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		user := "cluster"
		host := args[0]
		log.Println("Host " + host)
		publicKeyName := ".ssh/id_rsa.pub"
		privateKeyName := ".ssh/id_rsa"
		home, err := homedir.Dir()
		CheckErr(err)
		var publicKeyFile string
		var privateKeyFile string
		if len(home) > 0 {
			publicKeyFile = filepath.Join(home, ".ssh/id_rsa.pub")
			privateKeyFile = filepath.Join(home, ".ssh/id_rsa")
		} else {
			publicKeyFile = appendChildToExecutablePath(publicKeyName)
			privateKeyFile = appendChildToExecutablePath(privateKeyName)
		}
		if !checkFileExistence(publicKeyFile) && !checkFileExistence(privateKeyFile) {
			bitSize := 4096
			err := generateKeysAndWriteToFile(bitSize, privateKeyFile, publicKeyFile)
			CheckErr(err)
		} else {
			log.Println("Keys already exist")
		}
		log.Println("Connecting to remote servers root with password..")
		sshConfig := &ssh.ClientConfig{
			User:            "root",
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			//pass password from args?
			Auth: []ssh.AuthMethod{ssh.Password("pas")},
		}
		//add user "cluster"
		execSshCommand(host, "adduser --disabled-password --gecos \"\" "+user, sshConfig)
		log.Println("New user " + user + " added")
		//generate random password for user cluster
		pass := generateRandomString(32)
		execSshCommand(host, "echo \""+user+":"+pass+"\" | sudo chpasswd", sshConfig)
		execSshCommand(host, "usermod -aG sudo "+user, sshConfig)
		log.Println("Sudo permissions given to " + user)
		sshConfig = &ssh.ClientConfig{
			User:            user,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			//pass password from args?
			Auth: []ssh.AuthMethod{ssh.Password(pass)},
		}
		log.Println("Relogin from root to " + user)
		execSshCommand(host, "sudo passwd -l root", sshConfig)
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
		execSshCommand(host, "sudo sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/g' /etc/ssh/sshd_config",
			sshConfig)
		log.Println(user + " password auth disabled")
		pemBytes, err = ioutil.ReadFile(privateKeyFile)
		CheckErr(err)
		signer, err := ssh.ParsePrivateKey(pemBytes)
		CheckErr(err)
		log.Println("Connecting to remote servers " + user + " with public key")
		sshConfig = &ssh.ClientConfig{
			User:            user,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		}
		execSshCommand(host, "sudo ufw allow OpenSSH", sshConfig)
		execSshCommand(host, "sudo yes | sudo ufw enable", sshConfig)
		execSshCommand(host, "sudo ufw reload", sshConfig)
		log.Println("Firewall reloaded to work with OpenSSH")
	},
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
