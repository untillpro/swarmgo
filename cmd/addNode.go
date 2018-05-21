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
	"os"
	"path"
	"fmt"
	"os/exec"
)

// addNodeCmd represents the addNode command
var addNodeCmd = &cobra.Command{
	Use:   "add",
	Short: "add <ip>",
	Long: `Add node with current IP to cluster and do some installations???`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		go func(ip string) {
			out := exec.Command("ssh", "root@" + ip)
			fmt.Println(out)
		}(args[0])
		publicKeyName := ".ssh/id_rsa.pub"
		privateKeyName := ".ssh/id_rsa"
		home := os.Getenv("HOME")
		var publicKeyFile string
		var privateKeyFile string
		if len(home) > 0 {
			publicKeyFile = path.Join(home, ".ssh/id_rsa.pub")
			privateKeyFile = path.Join(home, ".ssh/id_rsa")
		} else {
			publicKeyFile = appendChildToExecutablePath(publicKeyName)
			privateKeyFile = appendChildToExecutablePath(privateKeyName)
		}
		if !checkFileExistence(publicKeyFile) && !checkFileExistence(privateKeyFile) {
			bitSize := 4096
			err := generateKeysAndWriteToFile(bitSize, privateKeyFile, publicKeyFile)
			if err != nil {
				panic(err)
			}
		}

		//dir, err := ioutil.TempDir("", "cluster")
		//if err != nil {
		//	log.Fatal(err)
		//	panic(err)
		//}
		//defer os.RemoveAll(dir)

		//change port to ssh service
		//b, err := ioutil.ReadFile("input.txt")
		//if err != nil {
		//	panic(err)
		//}


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
