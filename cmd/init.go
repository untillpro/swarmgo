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
	"strings"
	"io/ioutil"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Clusterfile",
	Long: `Initialize Clusterfile with list of used in project technologies and versions, in first version of swarmgo
	products and versions was hardcoded in future release it will be configures with flags`,
	Run: func(cmd *cobra.Command, args []string) {
		clusterFileName := "Clusterfile"
		clusterFile := appendChildToExecutablePath(clusterFileName)
		if checkFileExistence(clusterFile) {
			fmt.Println("Cluster already initialized!")
			return
		}
		zookeeper := ProductAndVersion{"zookeeper", "zookeeper-3.4.12"}
		docker := ProductAndVersion{"docker-ce", "17.12.0~ce-0~ubuntu"}
		prometheus := ProductAndVersion{"prometheus", "prometheus-2.2.1.linux-amd64"}
		grafana := ProductAndVersion{"grafana", "grafana_5.1.2_amd64"}
		traefik := ProductAndVersion{"traefik", "https://github.com/containous/traefik/releases/download/v1.6.0/traefik_linux-amd64"}
		f := func (pav ...ProductAndVersion) string {
			strs := make([]string, len(pav))
			for i := range pav {
			strs[i] = pav[i].String()
		}
			return strings.Join(strs, "\n")
		}
		b := []byte(f(zookeeper, docker, prometheus, grafana, traefik))
		err := ioutil.WriteFile(clusterFile, b, 0644)
		CheckErr(err)
		fmt.Println("Successful initialization")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// initCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// initCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
