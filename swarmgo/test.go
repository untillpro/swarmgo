package swarmgo

import (
	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
	"golang.org/x/crypto/ssh"
)

type NodesMeta struct {
	ClusterFile *clusterFile
	Leader      string
}

type SshContext struct {
	PassToKey string
	Leader    string
	conn      *ssh.ClientConfig
}

type LoggedRunnable func(args []string)

func logged(f LoggedRunnable) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		initCommand(cmd.Name())
		f(args)
		finitCommand()
	}
}

func readNodes() NodesMeta {
	firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
	return NodesMeta{
		ClusterFile: clusterFile,
		Leader:      firstEntry.node.Host,
	}
}

func sshSession(meta *NodesMeta) SshContext {
	ctx := SshContext{}
	ctx.PassToKey = readKeyPassword()
	ctx.conn = findSSHKeysAndInitConnection(ctx.PassToKey, meta.ClusterFile)
	return ctx
}

func (ctx *SshContext) sudoExec(host string, cmd string) string {
	return sudoExecSSHCommand(host, cmd, ctx.conn)
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test task",
	Long:  `Test test test`,
	Run: logged(func(args []string) {
		var nodes = readNodes()
		var sshCtx = sshSession(&nodes)
		var out = sshCtx.sudoExec(nodes.Leader, "docker stack deploy -c swarmprom/swarmprom.yml prom")
		gc.Info("Success:" + out)
	}),
}
