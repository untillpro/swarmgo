package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kballard/go-shellquote"
	gc "github.com/untillpro/gochips"
)

// SSHClient implements a simple SSH client working over standard exec.Command
type SSHClient struct {
	User                  string
	PrivateKeyFile        string
	StrictHostKeyChecking bool
	Verbose               bool
	HideStdout            bool
	Password              string
	TempDir               string
}

func checkSSHAgent() {
	output := os.Getenv("SSH_AUTH_SOCK")
	gc.Verbose("SSH_AUTH_SOCK", string(output))
	gc.ExitIfFalse(len(output) > 0, "ssh-agent isn't running")
}

// Client returns the SSHClient struct
func Client(user string, privateKey string) *SSHClient {
	return &SSHClient{
		User:                  user,
		PrivateKeyFile:        privateKey,
		StrictHostKeyChecking: true,
		Verbose:               false,
	}
}

func (c *SSHClient) prefixed(host, str string) string {
	return "SSHClient [" + c.User + "@" + host + "]: " + str
}

func (c *SSHClient) cmd(cmd *exec.Cmd) (string, error) {
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	var bufErr bytes.Buffer
	var bufOut bytes.Buffer

	teeOut := io.TeeReader(stdout, &bufOut)
	teeErr := io.TeeReader(stderr, &bufErr)

	err = cmd.Start()
	if err != nil {
		return "Unable to start command", err
	}

	go io.Copy(os.Stdout, teeOut)
	go io.Copy(os.Stderr, teeErr)

	err = cmd.Wait()
	if err != nil {
		stdErrStr := bufErr.String()
		if stdErrStr != "" {
			err = errors.New(err.Error() + " / " + stdErrStr)
		}
		return stdErrStr, err
	}

	out := strings.TrimRight(bufOut.String(), "\r\n")
	return out, nil
}

func (c *SSHClient) cmdHidden(cmd *exec.Cmd) (string, error) {

	var bErr bytes.Buffer
	cmd.Stderr = &bErr

	bs, err := cmd.Output()

	if err != nil {
		stdErrStr := bErr.String()
		if len(stdErrStr) > 0 {
			err = errors.New(err.Error() + " / " + stdErrStr)
		}
		return stdErrStr, err
	}

	out := strings.TrimRight(string(bs), "\r\n")
	return out, nil
}

func (c *SSHClient) loggedCmd(host string, cmd *exec.Cmd, maskInput, maskOutput bool) (string, error) {

	if c.Verbose {
		loggedInput := strings.Join(cmd.Args, " ")
		if maskInput {
			loggedInput = loggedInput[0:4] + "**(masked)**"
		}
		gc.Verbose(c.prefixed(host, loggedInput))
	}

	var err error
	var out string

	if c.HideStdout {
		out, err = c.cmdHidden(cmd)
	} else {
		out, err = c.cmd(cmd)
	}

	if c.Verbose {
		if err != nil {
			gc.Verbose(c.prefixed(host, "failed: "+err.Error()))
		} else {
			if maskOutput {
				gc.Verbose(c.prefixed(host, "success"), "**(masked)**")
			} else {
				gc.Verbose(c.prefixed(host, "success"), out)
			}
		}
	}

	return out, err
}

func (c *SSHClient) isMasked(cmd string) (command string, maskInput, maskOutput bool) {
	maskInput = strings.HasPrefix(cmd, "!") || strings.HasPrefix(cmd, "&")
	maskOutput = strings.HasPrefix(cmd, "$") || strings.HasPrefix(cmd, "&")
	if maskInput || maskOutput {
		command = cmd[1:]
	} else {
		command = cmd
	}
	return command, maskInput, maskOutput
}

// Exec executes the SSH command. The following prefixes can be added clarify verbose:
//    ! - mask verbosed input
//    $ - mask verbosed output
//    & - mask verbosed input & output
func (c *SSHClient) Exec(host string, command string) (string, error) {
	args := make([]string, 0)
	args = append(args, c.User+"@"+host)
	if !c.StrictHostKeyChecking {
		args = append(args, "-o StrictHostKeyChecking=no")
	}
	if len(c.PrivateKeyFile) > 0 {
		args = append(args, "-i")
		args = append(args, c.PrivateKeyFile)
	}
	command, maskInput, maskOutput := c.isMasked(command)
	args = append(args, command)
	var cmd *exec.Cmd
	if len(c.Password) > 0 {
		if runtime.GOOS == "windows" {

			tmp := filepath.Join(c.TempDir, fmt.Sprintf("%s@%s-cmd", c.User, host))
			ioutil.WriteFile(tmp, []byte(command), os.ModePerm) // write command(s) to file

			argsPas := make([]string, 0)
			argsPas = append(argsPas, "-batch")
			argsPas = append(argsPas, "-no-antispoof") // TODO: check this option
			argsPas = append(argsPas, "-ssh")
			argsPas = append(argsPas, "-pw")
			argsPas = append(argsPas, c.Password)
			argsPas = append(argsPas, "-m")
			argsPas = append(argsPas, tmp)
			argsPas = append(argsPas, fmt.Sprintf("%s@%s", c.User, host))

			cmd = exec.Command("plink", argsPas[:]...)
		} else {
			argsPas := make([]string, 0) // TODO: test on non-Windows platform
			argsPas = append(argsPas, "-p"+c.Password)
			argsPas = append(argsPas, "ssh")
			argsPas = append(argsPas, args...)
			cmd = exec.Command("sshpass", argsPas[:]...)
		}
	} else {
		cmd = exec.Command("ssh", args[:]...)
	}

	return c.loggedCmd(host, cmd, maskInput, maskOutput)
}

// ExecOrExit executes SSH command and terminates program execution with status (1) in case of any error
func (c *SSHClient) ExecOrExit(host string, command string) string {
	out, err := c.Exec(host, command)
	gc.ExitIfError(err)
	return out
}

// Copy copies local file to host by SSH
func (c *SSHClient) Copy(host string, size int64, mode os.FileMode, fileName string, contents io.Reader, destinationPath string) error {
	return c.copy(host, size, mode, fileName, contents, destinationPath)
}

// CopyPath copies local path to host by SSH
func (c *SSHClient) CopyPath(host string, filePath, destinationPath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	s, err := f.Stat()
	if err != nil {
		return err
	}
	if c.Verbose {
		gc.Verbose(c.prefixed(host, fmt.Sprintf("Copying %d bytes from [%s] to [%s]", s.Size(), filePath, destinationPath)))
	}
	return c.copy(host, s.Size(), s.Mode().Perm(), path.Base(filePath), f, destinationPath)
}

func (c *SSHClient) copy(host string, size int64, mode os.FileMode, fileName string, contents io.Reader, destination string) error {
	command := shellquote.Join("scp", "-t", destination)

	args := make([]string, 0)
	args = append(args, c.User+"@"+host)
	if !c.StrictHostKeyChecking {
		args = append(args, "-o StrictHostKeyChecking=no")
	}
	if len(c.PrivateKeyFile) > 0 {
		args = append(args, "-i")
		args = append(args, c.PrivateKeyFile)
	}
	args = append(args, command)
	cmd := exec.Command("ssh", args[:]...)

	//	defer session.Close()
	w, err := cmd.StdinPipe()

	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		w.Close()
		return err
	}

	errors := make(chan error)

	go func() {
		errors <- cmd.Wait()
	}()

	fmt.Fprintf(w, "C%#o %d %s\n", mode, size, fileName)
	io.Copy(w, contents)
	fmt.Fprint(w, "\x00")
	w.Close()

	return <-errors
}
