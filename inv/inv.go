/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package inv

import (
	"golang.org/x/crypto/ssh"
	"os/exec"
	"bytes"
	"strconv"
	"fmt"
	"net"
	"golang.org/x/crypto/ssh/agent"
)

func SSHAgent() ssh.AuthMethod {
	bin, err := exec.LookPath("ssh-agent")
	if err != nil {
		panic(err)
	}
	cmd := exec.Command(bin, "-s")
	out, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	fields := bytes.Split(out, []byte(";"))
	line := bytes.SplitN(fields[0], []byte("="), 2)
	line[0] = bytes.TrimLeft(line[0], "\n")
	if string(line[0]) != "SSH_AUTH_SOCK" {
		panic(err)
	}
	socket := string(line[1])
	line = bytes.SplitN(fields[2], []byte("="), 2)
	line[0] = bytes.TrimLeft(line[0], "\n")
	if string(line[0]) != "SSH_AGENT_PID" {
		panic(err)
	}
	pidStr := line[1]
	pid, err := strconv.Atoi(string(pidStr))
	if err != nil {
		panic(err)
	}
	fmt.Println(pid)
	conn, err := net.Dial("unix", string(socket))
	if err != nil {
		panic(err)
	}

	ac := agent.NewClient(conn)
	return ssh.PublicKeysCallback(ac.Signers)
	//if sshAgent, err := net.Dial("unix", socket); err == nil {
	//	return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	//}
	//return nil
}
