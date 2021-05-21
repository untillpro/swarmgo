/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cli

import (
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize swarmgo-config",
	Long:  `Initialize swarmgo-config.yml with list of used in project technologies and versions`,
	Run: loggedCmd(func(cmd *cobra.Command, args []string) {
		clusterFilePath := filepath.Join(getWorkingDir(), swarmgoConfigFileName)
		gc.ExitIfFalse(!FileExists(clusterFilePath), "swarmgo-config.yml already created")

		defaultClusterFileRelativePath := filepath.Join(swarmgoCliFolder, swarmgoConfigFileName)
		gc.ExitIfFalse(FileExists(defaultClusterFileRelativePath), "You should clone swarmgo-config.yml from repo!")

		clusterFile := clusterFile{
			OrganizationName: queryUserInput("Enter your organization name:"),
			ClusterName:      queryUserInput("Enter your cluster name:"),
		}

		configEntry := executeTemplateToFile(defaultClusterFileRelativePath, clusterFile)
		gc.ExitIfError(ioutil.WriteFile(clusterFilePath, configEntry.Bytes(), swarmgoConfigPerms))
		gc.Info("Done: swarmgo-config.yml created in " + getWorkingDir() + " folder, perhaps you will have to modify some variables")
	}),
}
