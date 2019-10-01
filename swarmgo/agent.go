package swarmgo

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Starts ssh-agent and adds cluster key with ssh-add to current terminal session",
	Long:  `For Shell terminals only. Use this command to input key password once and avoid typing it on further swarmgo commands in current temrinal session`,
	Run: loggedCmd(func(args []string) {

		output, err := exec.Command("ssh-agent").Output()
		gc.ExitIfError(err, "Unable to run ssh-agent")
		gc.Verbose("ssh-agent output", string(output))
		sshAgentOut := output

		re := regexp.MustCompile("((?:SSH_AUTH_SOCK|SSH_AGENT_PID)=[^;]*)")
		submatch := re.FindAllStringSubmatch(string(output), -1)

		gc.ExitIfFalse(submatch != nil && len(submatch) == 2 && len(submatch[0]) == 2 && len(submatch[1]) == 2,
			"Expected SSH_AUTH_SOCK and SSH_AGENT_PID in ssh-agent output, see logs for more details")

		clusterFile := unmarshalClusterYml()
		_, privateKeyFile := findSSHKeys(clusterFile)
		gc.ExitIfFalse(FileExists(privateKeyFile), "Private key file doesn't exist: "+privateKeyFile)
		gc.Verbose("Private Key location:", privateKeyFile)

		cmd := exec.Command("env", submatch[0][1], submatch[1][1], "ssh-add", privateKeyFile)
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
		gc.Verbose("Running", cmd.Path, cmd.Args)
		err = cmd.Run()
		gc.ExitIfError(err, "Unable to run ssh-add")

		fmt.Printf(string(sshAgentOut))
	}),
}
