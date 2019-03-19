package swarmgo

import (
	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize swarmgo-config",
	Long:  `Initialize swarmgo-config.yml with list of used in project technologies and versions`,
	Run: func(cmd *cobra.Command, args []string) {
		initCommand("init")
		defer finitCommand()
		clusterFilePath := filepath.Join(getCurrentDir(), swarmgoConfigFileName)
		if FileExists(clusterFilePath) {
			gc.Info("swarmgo-config.yml already initialized!")
			return
		}
		clusterFile := unmarshalDefaultClusterYml()
		gc.Info("Enter your organization name")
		clusterFile.OrganizationName = waitUserInput()
		gc.Info("Enter your cluster name")
		clusterFile.ClusterName = waitUserInput()
		out, err := yaml.Marshal(&clusterFile)
		CheckErr(err)
		err = ioutil.WriteFile(clusterFilePath, out, 0644)
		CheckErr(err)
		gc.Info("swarmgo-config.yml created in root folder, check products versions and modify it if needed")
	},
}

func unmarshalDefaultClusterYml() *clusterFile {
	defaultClusterFileRelativePath := filepath.Join("swarmgo", swarmgoConfigFileName)
	clusterFileEntry := readFileIfExists(defaultClusterFileRelativePath, "You should clone swarmgo-config.yml from repo!")
	clusterFileStruct := clusterFile{}
	err := yaml.Unmarshal(clusterFileEntry, &clusterFileStruct)
	CheckErr(err)
	return &clusterFileStruct
}
