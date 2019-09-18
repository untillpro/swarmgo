/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mitchellh/go-homedir"
	gc "github.com/untillpro/gochips"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v2"
)

func readPasswordPrompt(prompt string) string {
	fmt.Print(prompt + ":")
	password, err := terminal.ReadPassword(int(syscall.Stdin))
	gc.ExitIfError(err)
	fmt.Println("")
	return string(password)
}

func readKeyPassword() string {
	return readPasswordPrompt("Key password")
}

func appendChildToExecutablePath(child string) string {
	current, err := os.Executable()
	gc.ExitIfError(err)
	return filepath.Join(filepath.Dir(current), child)
}

// FileExists s.e.
func FileExists(clusterFile string) bool {
	_, err := os.Stat(clusterFile)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	gc.ExitIfError(err)
	return true
}

func logWithPrefix(prefix, str string) {
	gc.Info(prefix + ": " + str)
}

func doingWithPrefix(prefix, str string) {
	gc.Doing(prefix + ": " + str)
}

func redirectLogs() *os.File {
	parent := filepath.Join(getSourcesDir(), "logs")
	gc.ExitIfError(os.MkdirAll(parent, os.ModePerm))
	f, err := os.OpenFile(filepath.Join(parent, "log.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	gc.ExitIfError(err)
	log.SetOutput(f)
	return f
}

func sudoExecSSHCommand(host, cmd string, config *ssh.ClientConfig) string {
	return execSSHCommand(host, "sudo "+cmd, config)
}

func sudoExecSSHCommandWithoutPanic(host, cmd string, config *ssh.ClientConfig) (string, error) {
	return execSSHCommandWithoutPanic(host, "sudo "+cmd, config)
}

func execSSHCommand(host, cmd string, config *ssh.ClientConfig) string {
	bs, err := execSSHCommandWithoutPanic(host, cmd, config)
	gc.ExitIfError(err, string(bs))
	return string(bs)
}

type singleWriter struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func execSSHCommandWithoutPanic(host, cmd string, config *ssh.ClientConfig) (string, error) {
	if strings.HasPrefix(cmd, "!") {
		cmd = cmd[1:]
	} else {
		gc.Verbose("SSH", host+", "+cmd)
	}
	conn, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		gc.Verbose("SSH:Dial failed", "")
		return "", err
	}
	session, err := conn.NewSession()
	if err != nil {
		gc.Verbose("SSH:Session creation failed", "")
		return "", err
	}
	defer session.Close()
	var bErr bytes.Buffer
	session.Stderr = &bErr

	bs, err := session.Output(cmd)
	if err != nil {
		stdErrStr := bErr.String()
		if stdErrStr != "" {
			err = errors.New(err.Error() + " / " + stdErrStr)
		}
		gc.Verbose("SSH:session.CombinedOutput failed", stdErrStr, string(bs))
		return string(bs), err
	}
	return string(bs), nil
}

func getSSHSession(host string, config *ssh.ClientConfig) *ssh.Session {
	conn, err := ssh.Dial("tcp", host+":22", config)
	gc.ExitIfError(err)
	session, err := conn.NewSession()
	gc.ExitIfError(err)
	return session
}

func initSSHConnectionConfigWithPublicKeys(userName, privateKeyFile, password string) *ssh.ClientConfig {
	pemBytes, err := ioutil.ReadFile(privateKeyFile)
	gc.ExitIfError(err)
	signer, err := ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(password))
	gc.ExitIfError(err)
	sshConfig := &ssh.ClientConfig{
		User:            userName,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}
	return sshConfig
}

func contains(slice []string, find string) bool {
	for _, a := range slice {
		if a == find {
			return true
		}
	}
	return false
}

func containsNode(slice []node, find node) bool {
	for _, a := range slice {
		if a == find {
			return true
		}
	}
	return false
}

func readWorkingFileIfExists(fileName, errorMessage string) []byte {
	nodesFileEntry, err := ioutil.ReadFile(filepath.Join(getWorkingDir(), fileName))
	if err != nil {
		if os.IsNotExist(err) {
			gc.Error(err)
			gc.Error(errorMessage)
			os.Exit(1)
		}
	}
	return nodesFileEntry
}

func waitUserInput() string {
	buf := bufio.NewReader(os.Stdin)
	input, err := buf.ReadString('\n')
	gc.ExitIfError(err)
	return strings.Trim(input, "\n\r ")
}

func queryUserInput(prompt string) string {
	fmt.Print(prompt)
	return waitUserInput()
}

func findSSHKeys(clusterFile *clusterFile) (string, string) {
	var publicKeyFile string
	var privateKeyFile string
	if len(clusterFile.PublicKey) != 0 && len(clusterFile.PrivateKey) != 0 {
		publicKeyFile = clusterFile.PublicKey
		privateKeyFile = clusterFile.PrivateKey
	} else {
		publicKeyFile = ".ssh/" + clusterFile.ClusterName + ".pub"
		privateKeyFile = ".ssh/" + clusterFile.ClusterName
		home, err := homedir.Dir()
		gc.ExitIfError(err)
		if len(home) > 0 {
			publicKeyFile = filepath.Join(home, publicKeyFile)
			privateKeyFile = filepath.Join(home, privateKeyFile)
		} else {
			publicKeyFile = appendChildToExecutablePath(publicKeyFile)
			privateKeyFile = appendChildToExecutablePath(privateKeyFile)
		}
	}
	return publicKeyFile, privateKeyFile
}

func convertStringToInt(s string) int {
	convertExit, err := strconv.Atoi(strings.TrimSuffix(s, "\n"))
	gc.ExitIfError(err)
	return convertExit
}

func getNodesFromYml(parentFolderName string) []node {
	nodesFileName := filepath.Join(parentFolderName, nodesFileName)
	nodesFile, err := os.OpenFile(nodesFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	gc.ExitIfError(err)
	nodesFromYaml := make([]node, 0, 5)
	fileEntry, err := ioutil.ReadAll(nodesFile)
	gc.ExitIfError(err)
	err = yaml.Unmarshal(fileEntry, &nodesFromYaml)
	gc.ExitIfError(err)
	return nodesFromYaml
}

func numberHostsFromNodesFile(nodesFromYml []node) string {
	hostsWithNumbers := make(map[int]string, len(nodesFromYml))
	for i := range nodesFromYml {
		hostsWithNumbers[i] = nodesFromYml[i].Alias
		i++
	}
	gc.Info("Please choose number of node from `nodesFileName`")
	return inputFuncForHosts(hostsWithNumbers)
}

func inputFuncForHosts(hostsWithNumbers map[int]string) string {
	b := new(bytes.Buffer)
	for k, v := range hostsWithNumbers {
		fmt.Printf("%d : %s\n", k, v)
	}
	gc.Info("\n" + b.String())
	input := waitUserInput()
	convertedInput := convertStringToInt(input)
	if value, ok := hostsWithNumbers[convertedInput]; ok {
		return value
	}
	gc.Info("Wrong number, specifys one of this!")
	return inputFuncForHosts(hostsWithNumbers)
}

func unmarshalClusterYml() *clusterFile {
	clusterFileEntry := readWorkingFileIfExists(swarmgoConfigFileName, "You should create swarmgo-config.yml")
	clusterFileStruct := clusterFile{}
	gc.ExitIfError(yaml.Unmarshal(clusterFileEntry, &clusterFileStruct))
	return &clusterFileStruct
}

func findSSHKeysAndInitConnection(passToKey string, config *clusterFile) *ssh.ClientConfig {
	_, privateKeyFile := findSSHKeys(config)
	gc.ExitIfFalse(FileExists(privateKeyFile), "Can't find private key to connect to remote server!")
	return initSSHConnectionConfigWithPublicKeys(config.ClusterUserName, privateKeyFile, passToKey)
}

func substringAfterIncludeValue(value string, a string) string {
	pos := strings.LastIndex(value, a) - len(a)
	if pos <= -1 {
		return ""
	}
	adjustedPos := pos + len(a)
	if adjustedPos >= len(value) {
		return ""
	}
	return value[adjustedPos:]
}

func substringAfter(value string, a string) string {
	pos := strings.LastIndex(value, a)
	if pos <= -1 {
		return ""
	}
	adjustedPos := pos + len(a)
	if adjustedPos >= len(value) {
		return ""
	}
	return value[adjustedPos:]
}

var workingDirReported bool

func getWorkingDir() string {
	src := getSourcesDir()
	res := src
	defer func() {
		if !workingDirReported {
			gc.Verbose("workingDir", res)
			workingDirReported = true
		}
	}()
	nodesFile := filepath.Join(src, nodesFileName)
	if FileExists(nodesFile) {
		return res
	}

	res = path.Join(src, ".nodes")
	if FileExists(res) {
		return res
	}

	res = path.Join(src, "nodes")
	if FileExists(res) {
		return res
	}
	gc.ExitIfError(os.MkdirAll(res, 0777))
	return res
}

func getSourcesDir() string {
	pwd, err := os.Getwd()
	gc.ExitIfError(err)
	return pwd
}

func generateRandomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
