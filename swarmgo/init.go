package swarmgo

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize swarmgo-config",
	Long:  `Initialize swarmgo-config.yml with list of used in project technologies and versions`,
	Run: func(cmd *cobra.Command, args []string) {
		initCommand("init")
		defer finitCommand()
		clusterFilePath := filepath.Join(getWorkingDir(), swarmgoConfigFileName)
		if FileExists(clusterFilePath) {
			gc.Info("swarmgo-config.yml already created")
			return
		}
		clusterFile := clusterFile{}
		fmt.Print("Enter your organization name:")
		clusterFile.OrganizationName = waitUserInput()
		fmt.Print("Enter your cluster name:")
		clusterFile.ClusterName = waitUserInput()
		defaultClusterFileRelativePath := filepath.Join("swarmgo", swarmgoConfigFileName)
		if !FileExists(defaultClusterFileRelativePath) {
			gc.Fatal("You should clone swarmgo-config.yml from repo!")
		}
		configEntry := executeTemplateToFile(defaultClusterFileRelativePath, clusterFile)
		err := ioutil.WriteFile(clusterFilePath, configEntry.Bytes(), 0644)
		CheckErr(err)
		gc.Info("swarmgo-config.yml created in " + getWorkingDir() + " folder, perhaps you will have to modify some variables")
	},
}
