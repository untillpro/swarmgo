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
	"log"
)

const docker = "docker-ce"

// dockerCmd represents the docker command
var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Install docker",
	Long:  `Downloads and installs docker specific version. Version takes from Clusterfile`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Starting docker installation...")
		version := findDockerVersionFromClusterfile()
		nodesFileEntry := readNodesFileIfExists()
		host := takeHostFromFlagOrChooseFromNodesFile(nodesFileEntry)
		config := findSshKeysAndInitConnection()
		if checkDockerInstallation(host, version ,config) {
			log.Fatal("Docker version " + version + " already installed!")
		}
		log.Println("Updating apt-get...")
		sudoExecSshCommand(host, "apt-get update", config)
		log.Println("Installing packages to allow apt to use a repository over HTTPS...")
		sudoExecSshCommand(host, "apt-get -y install apt-transport-https ca-certificates curl "+
			"software-properties-common", config)
		//TODO don't forget auto-check
		log.Println("Add Docker’s official GPG key")
		execSshCommand(host, "curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -", config)
		log.Println("Verify key")
		log.Println(sudoExecSshCommand(host, "apt-key fingerprint 0EBFCD88", config))
		log.Println("Adding repository")
		sudoExecSshCommand(host, "add-apt-repository \"deb [arch=amd64] https://download.docker.com/linux/ubuntu "+
			"$(lsb_release -cs) stable\"", config)
		log.Println("Updating apt-get...")
		sudoExecSshCommand(host, "apt-get update", config)
		log.Println("Trying to install " + docker + " version " + version + "...")
		sudoExecSshCommand(host, "apt-get -y install "+docker+"="+version, config)
		log.Println("Checking installation...")
		if checkDockerInstallation(host, version ,config) {
			log.Println("Docker succesfully installed!")
		} else {
			log.Fatal("Can't install docker")
		}
	},
}

func init() {
	nodeCmd.AddCommand(dockerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// dockerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
}
