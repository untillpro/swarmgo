package swarmgo

import (
	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

var privateKeyArg string
var publicKeyArg string

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Initialize keys for ssh access",
	Long:  `Generate new keys or specify keys for ssh access configuration.`,
	Run: loggedCmd(func(args []string) {

		gc.Doing("Reading config")

		readWorkingFileIfExists(swarmgoConfigFileName, "Config file not found, to create it run `swarmgo init`")
		clusterFile := unmarshalClusterYml()

		gc.Doing("Checking keys")

		if privateKeyArg != "" || publicKeyArg != "" {
			gc.ExitIfFalse(privateKeyArg != "" && publicKeyArg != "", "Both private and public keys need to be specified")
			gc.ExitIfFalse(FileExists(privateKeyArg), "Specified private key doesn't exist")
			gc.ExitIfFalse(FileExists(publicKeyArg), "Specified public key doesn't exist")
			clusterFile.PrivateKey = privateKeyArg
			clusterFile.PublicKey = publicKeyArg
			marshalClusterYml(clusterFile)
			gc.Info("Private and public keys updated in configuration file: " + swarmgoConfigFileName)
			gc.Info("PrivateKey: " + clusterFile.PrivateKey)
			gc.Info("PublicKey: " + clusterFile.PublicKey)
		} else {
			publicKeyFile, privateKeyFile := findSSHKeys(clusterFile)
			gc.Info("Public Key location:", publicKeyFile)
			gc.Info("Private Key location:", privateKeyFile)

			filesExist := FileExists(publicKeyFile) && FileExists(privateKeyFile)

			if !filesExist {
				gc.Doing("Generating new keys")
				passToKey := readKeyPassword()
				bitSize := 4096
				err := generateKeysAndWriteToFile(bitSize, privateKeyFile, publicKeyFile, passToKey)
				gc.ExitIfError(err)
			} else {
				gc.Info("Keys already configured")
			}
		}
	}),
}
