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
	"log"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"strings"
)

var mode string

const (
	workerTokenFileName  = "worker"
	managerTokenFileName = "manager"
)

// swarmCmd represents the swarm command
var swarmCmd = &cobra.Command{
	Use:   "swarm <arg> [#flag]",
	Short: "Choose --mode= [manager, worker], empty means `init`",
	Long:  `Initialize or joins to swarm for given node with default params`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("Start swarm execution...")
		version := findDockerVersionFromClusterfile()
		nodesFileEntry := readNodesFileIfExists()
		var host string
		if len(args) == 1 {
			host = args[0]
		} else {
			host = takeHostsFromArgsOrChooseFromNodesFile(nodesFileEntry, args)[0]
		}
		config := findSshKeysAndInitConnection()
		if !checkDockerInstallation(host, version, config) {
			log.Fatal("Need to install docker " + version + " before init swarm")
		}
		if checkSwarmExistence(host, config) {
			log.Fatal("Node already in swarm")
		}
		log.Println("Configuring ufw to work with swarm...")
		sudoExecSshCommand(host, "ufw allow 22/tcp", config)
		sudoExecSshCommand(host, "ufw allow 2376/tcp", config)
		sudoExecSshCommand(host, "ufw allow 7946/tcp", config)
		sudoExecSshCommand(host, "ufw allow 7946/udp", config)
		sudoExecSshCommand(host, "ufw allow 4789/udp", config)
		log.Println("Ufw configured")
		reloadUfwAndDocker := func() {
			log.Println("Restarting ufw and docker...")
			sudoExecSshCommand(host, "ufw reload", config)
			sudoExecSshCommand(host, "systemctl restart docker", config)
			log.Println("Ufw and docker restarted!")
		}
		switch mode {
		case "":
			sudoExecSshCommand(host, "ufw allow 2377/tcp", config)
			reloadUfwAndDocker()
			initSwarm(host, config)
		case "manager":
			sudoExecSshCommand(host, "ufw allow 2377/tcp", config)
			reloadUfwAndDocker()
			managerFile := appendChildToExecutablePath(managerTokenFileName)
			joinToSwarm(host, managerFile, config)
		case "worker":
			reloadUfwAndDocker()
			workerFile := appendChildToExecutablePath(workerTokenFileName)
			joinToSwarm(host, workerFile, config)
		}
	},
}

func initSwarm(host string, config *ssh.ClientConfig) {
	log.Println("Starting swarm initialization...")
	sudoExecSshCommand(host, "docker swarm init --advertise-addr "+host+":2377", config)
	writeTokenToFile(workerTokenFileName, host, config)
	writeTokenToFile(managerTokenFileName, host, config)
}

func writeTokenToFile(mode, host string, config *ssh.ClientConfig) {
	output := sudoExecSshCommand(host, "docker swarm join-token "+mode, config)
	token := strings.Trim(substringAfterIncludeValue(output, "docker swarm join"), "\n ")
	workerFile := appendChildToExecutablePath(mode)
	err := ioutil.WriteFile(workerFile, []byte(token), 0644)
	CheckErr(err)
	log.Println(mode + " token wrote")
}

func joinToSwarm(host, file string, config *ssh.ClientConfig) {
	log.Println("Joining to swarm in " + mode + " mode...")
	token, err := ioutil.ReadFile(file)
	CheckErr(err)
	sudoExecSshCommand(host, string(token), config)
	log.Println("Successfully joined to swarm")
}

func init() {
	nodeCmd.AddCommand(swarmCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// swarmCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	swarmCmd.Flags().StringVarP(&mode, "mode", "m", "", "Swarm mode: empty means `init`,"+
		" `join-manager`, `join-worker`")
}
