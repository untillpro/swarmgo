package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

var luckyRootPassword string
var luckyMonPassword string
var luckyNoAlerts bool
var luckySlackWebhookURL string

const (
	traefikLabelValue    = "traefik=true"
	prometheusLabelValue = "prometheus=true"
)

func alias(i int) string {
	return fmt.Sprintf("node%d", i)
}

var imluckyCmd = &cobra.Command{
	Use:   "imlucky IP [IP] [IP]",
	Short: "Builds swarm cluster with automatically assigned settings",
	Long:  "Specify one, two or three nodes to build cluster with automatically assigned settings",
	Args:  cobra.RangeArgs(1, 3),
	Run: loggedCmd(func(cmd *cobra.Command, args []string) {

		gc.ExitIfFalse(len(getNodesFromYml(getWorkingDir())) == 0, "Some nodes already has been added already. Use 'imlucky' command on clean configuration only")

		checkSSHAgent()

		nodes := make(map[string]string)
		for i, arg := range args {
			nodeAlias := alias(i + 1)
			nodes[nodeAlias] = arg
		}

		AddNodes(nodes, luckyRootPassword)
		InstallDocker(false, []string{})
		if len(nodes) == 1 {
			AddToSwarm(true, []string{alias(1)})
			LabelAdd(alias(1), traefikLabelValue)
			LabelAdd(alias(1), prometheusLabelValue)
		} else if len(nodes) == 2 {
			AddToSwarm(true, []string{alias(1)})
			AddToSwarm(false, []string{alias(2)})
			LabelAdd(alias(2), traefikLabelValue)
			LabelAdd(alias(2), prometheusLabelValue)
		} else {
			AddToSwarm(true, []string{alias(1), alias(2), alias(3)})
			LabelAdd(alias(1), traefikLabelValue)
			LabelAdd(alias(2), prometheusLabelValue)
		}
		gc.Info("Current node labels:")
		LabelList()

		DeployTraefik(luckyMonPassword)
		DeploySwarmprom(luckyNoAlerts, luckySlackWebhookURL, luckyMonPassword, luckyMonPassword, luckyMonPassword)

		gc.Info("Done imlucky")

	}),
}
