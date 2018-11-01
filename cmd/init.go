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
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"path/filepath"
)

const (
	clusterFileName              = "clusterfile.yml"
	clusterNameDefaultValue      = `<Your cluster name>`
	grafanaAdminUserDefaultValue = "<Your user name>"
	slackUrlDefaultValue         = "<Your slack alertmanager webhook URL>"
)

type ClusterFile struct {
	OrganizationName string `yaml:"OrganizationName"`
	ClusterName      string `yaml:"ClusterName"`
	ClusterUser      string `yaml:"ClusterUser"`
	Docker           string `yaml:"Docker-ce"`
	Traefik          string `yaml:"Traefik"`
	GrafanaAdminUser string `yaml:"GrafanaAdminUser"`
	SlackUrl         string `yaml:"SlackUrl"`
	SlackChannelName string `yaml:"SlackChannelName"`
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Clusterfile",
	Long: `Initialize Clusterfile with list of used in project technologies and versions, in first version of swarmgo
	products and versions was hardcoded in future release it will be configures with flags`,
	Run: func(cmd *cobra.Command, args []string) {
		if logs {
			f := redirectLogs()
			defer f.Close()
		}
		clusterFile := filepath.Join(getCurrentDir(), clusterFileName)
		if checkFileExistence(clusterFile) {
			log.Println("Cluster already initialized!")
			return
		}
		clusterFileEntry := ClusterFile{
			"<Your organization name>",
			clusterNameDefaultValue,
			"cluster",
			"17.12.0~ce-0~ubuntu",
			"traefik:1.7",
			grafanaAdminUserDefaultValue,
			slackUrlDefaultValue,
			"cluster",
		}
		out, err := yaml.Marshal(&clusterFileEntry)
		CheckErr(err)
		err = ioutil.WriteFile(clusterFile, out, 0644)
		CheckErr(err)
		log.Println("Successful initialization")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
