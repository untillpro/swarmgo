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
	"strings"
	"io/ioutil"
)

const clusterFileName  = "Clusterfile"

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Clusterfile",
	Long: `Initialize Clusterfile with list of used in project technologies and versions, in first version of swarmgo
	products and versions was hardcoded in future release it will be configures with flags`,
	Run: func(cmd *cobra.Command, args []string) {
		clusterFile := appendChildToExecutablePath(clusterFileName)
		if checkFileExistence(clusterFile) {
			fmt.Println("Cluster already initialized!")
			return
		}
		zookeeper := "zookeeper=zookeeper-3.4.12"
		docker := "docker-ce=17.12.0~ce-0~ubuntu"
		prometheus := "prometheus=prometheus-2.2.1.linux-amd64"
		grafana := "grafana=grafana_5.1.2_amd64"
		traefik := "traefik=https://github.com/containous/traefik/releases/download/v1.6.0/traefik_linux-amd64"
		f := func(str ... string) string {
			return strings.Join(str, "\n")
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
