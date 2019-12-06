/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

const (
	nomadVersion = "0.10.2"
)

type nomadCfg struct {
	Host       string
	NodeName   string
	LeaderHost string
}

// InstallNomad installs consul
func InstallNomad(server bool, args []string) {
	clusterFile := unmarshalClusterYml()

	nodesFromYml := getNodesFromYml(getWorkingDir())
	gc.ExitIfFalse(len(nodesFromYml) > 0, "Can't find nodes from nodes.yml. Add some nodes first")

	// TODO: check that consul installed

	nodeHostAndNode := make(map[string]node)
	/*	for _, value := range nodesFromYml {
			nodeHostAndNode[value.Host] = value
		}

		clusterLeaderNode := node{}
		if server {
			clusterLeaderNode = nodesFromYml[0] // TODO: temporary solution!
		}
	*/

	var channelForNodes = make(chan nodeAndError)
	for _, currentNode := range nodesFromYml {
		go func(nodeVar node) {
			nodeFromGoroutine, err := installNomadAtNode(nodeVar, nodesFromYml[0].Host, clusterFile, server)
			nodeFromFunc := nodeAndError{
				nodeFromGoroutine,
				err,
			}
			channelForNodes <- nodeFromFunc
		}(currentNode)
	}

	errMsgs := make([]string, 0, len(args))
	for _, key := range nodesFromYml {
		nodeWithPossibleError := <-channelForNodes
		node := nodeWithPossibleError.nodeWithPossibleError
		err := nodeWithPossibleError.err
		if nodeWithPossibleError.err != nil {
			errMsgs = append(errMsgs, fmt.Sprintf("Host: %v, returns error: %v", node.Host,
				err.Error()))
		}
		nodeHostAndNode[key.Host] = node
	}
	for _, errMsg := range errMsgs {
		gc.Info(errMsg)
	}
	close(channelForNodes)

	/*	nodes := make([]node, len(nodeHostAndNode))
		i := 0
		for _, value := range nodeHostAndNode {
			nodes[i] = value
			i++
		}
		marshaledNode, err := yaml.Marshal(&nodes)
		gc.ExitIfError(err)

		nodesFilePath := filepath.Join(getWorkingDir(), nodesFileName)

		gc.ExitIfError(ioutil.WriteFile(nodesFilePath, marshaledNode, 0600))

		gc.ExitIfFalse(len(errMsgs) == 0, "Failed to install on some node(s)")*/
}

var nomadCmd = &cobra.Command{
	Use: "nomad -s <Alias1> <Alias2> installs nomad in server mode (experimental)",
	Run: func(cmd *cobra.Command, args []string) {
		initCommand("nomad")
		defer finitCommand()
		if mode && len(args) == 0 {
			gc.Fatal("Need at least one node alias")
		}
		checkSSHAgent()
		InstallNomad(mode, args)
	},
}

func configUfwForNomad(host string, client *SSHClient) error {
	commands := []SSHCommand{
		SSHCommand{
			cmd:   "sudo apt-get -y install ufw",
			title: "Installing firewall",
		},
		SSHCommand{
			title: "Setting up firewall rules",
			cmd:   "sudo ufw allow 4648/tcp", // serf wan
			cmd2:  "sudo ufw allow 4648/udp", // serf wan
			cmd3:  "sudo ufw allow 4647/tcp", // rpc
			cmd4:  "sudo ufw allow 4646/tcp", // http api
		},
		SSHCommand{ // DEV
			title: "Setting up firewall rules (dev)",
			cmd:   "sudo ufw allow 8000/tcp", // TODO: dev!
			cmd2:  "sudo ufw allow 8080/tcp", // TODO: dev!
		},
	}

	err := sshKeyAuthCmds(host, client, commands)
	if err != nil {
		return err
	}

	logWithPrefix(host, "Firewall configured")
	return nil
}

func installNomadAtNode(node node, leaderHost string, file *clusterFile, server bool) (node, error) {

	client := getSSHClient(file)

	err := configUfwForNomad(node.Host, client)
	if err != nil {
		return node, err
	}

	cfg := nomadCfg{
		Host:       node.Host,
		NodeName:   node.Alias,
		LeaderHost: leaderHost,
	}

	templateAndCopy(client, node.Host, "scripts/nomad.service", "~/nomad.service", cfg)
	templateAndCopy(client, node.Host, "scripts/nomad.hcl", "~/nomad.hcl", cfg)
	templateAndCopy(client, node.Host, "scripts/nomad-server.hcl", "~/nomad-server.hcl", cfg)

	templateAndCopy(client, node.Host, "scripts/nomad/traefik.job", "~/traefik.job", cfg) // TODO: dev!
	templateAndCopy(client, node.Host, "scripts/nomad/myapp.job", "~/myapp.job", cfg)     // TODO: dev!

	commands := []SSHCommand{
		SSHCommand{
			cmd:   fmt.Sprintf("curl --silent --remote-name https://releases.hashicorp.com/nomad/%s/nomad_%s_linux_amd64.zip", nomadVersion, nomadVersion),
			title: "Downloading nomad",
		},
		SSHCommand{
			cmd:   fmt.Sprintf("unzip nomad_%s_linux_amd64.zip", nomadVersion),
			title: "Unzipping package",
		},
		SSHCommand{
			title: "Moving to /usr/local/bin/",
			cmd:   "sudo chown root:root nomad",
			cmd2:  "sudo mv nomad /usr/local/bin/",
		},
		/*		SSHCommand{
				cmd:   "sudo useradd --system --home /etc/consul.d --shell /bin/false consul",
				title: "Creating user consul",
			}, */
		SSHCommand{
			title: "Creating user home dir",
			cmd:   "sudo mkdir --parents /opt/nomad",
		},
		SSHCommand{
			cmd:   "sudo chmod 777 /etc/systemd/system",
			cmd1:  "sudo cp ~/nomad.service /etc/systemd/system/nomad.service",
			title: "Creating service",
		},
		SSHCommand{
			title: "Creating nomad configuration file",
			cmd:   "sudo mkdir --parents /etc/nomad.d",
			cmd1:  "sudo chmod 777 /etc/nomad.d",
			cmd2:  "sudo cp ~/nomad.hcl /etc/nomad.d/nomad.hcl",
		},
		SSHCommand{
			title: "Creating server cfg file",
			cmd:   "sudo cp ~/nomad-server.hcl /etc/nomad.d/server.hcl",
		},
		SSHCommand{
			title: "Setting environment variables",
			cmd:   fmt.Sprintf("echo \"export NOMAD_ADDR=http://%s:4646\" >> ~/.profile", node.Host),
		},
		SSHCommand{
			title: "Starting nomad service",
			cmd:   "sudo systemctl enable nomad",
			cmd2:  "sudo systemctl start nomad",
		},
	}

	err = sshKeyAuthCmds(node.Host, client, commands)
	if err != nil {
		return node, err
	}

	/*	ver, err := client.Exec(node.Host, "consul --version")
		if ver != consulVersion {
			return node, fmt.Errorf("Couldn't install consul version %s", consulVersion)
		} */

	node.SwarmMode = manager
	logWithPrefix(node.Host, node.Alias+" nomad successfully deployed")
	return node, nil
}
