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
	"fmt"

	"github.com/spf13/cobra"
	"log"
	"os"
	"golang.org/x/crypto/ssh"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh <Alias>",
	Short: "Runner for commands through ssh",
	Long: `After use first time you need to enter password to private key. After that you can use commands without 
password before you don't use command exit'`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		readFileIfExists(nodesFileName, "Need to add some nodesFileName first")
		nodesFromYaml := getNodesFromYml(getCurrentDir())
		var host string
		for _, node := range nodesFromYaml {
			if node.Alias == args[0] {
				host = node.Host
				break
			}
		}
		if len(host) == 0 {
			log.Fatal("Alias doesn't present in nodes.yml!")
		}
		fmt.Println("Enter password to crypt/decrypt you private key")
		passToKey := waitUserInput()
		fmt.Println("Enter username")
		userName := waitUserInput()
		sshConfig := findSshKeysAndInitConnection(userName, passToKey)
		fmt.Println("Now you can execute commands")
		conn, err := ssh.Dial("tcp", host+":22", sshConfig)
		CheckErr(err)
		for {
			command := waitUserInput()
			switch command {
			case "":
			case "exit":
				os.Exit(0)
			default:
				go func() {
					out, err := execSshCommandWithoutPanic(command, conn)
					if err != nil {
						fmt.Print(err)
					}
					fmt.Print(out)
				}()
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
}
