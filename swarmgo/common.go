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
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
	yaml "gopkg.in/yaml.v2"
)

func assert(condition bool, info ...interface{}) {
	if !condition {
		log.Fatal(info...)
	}
}

func readPasswordPrompt(prompt string) string {
	fmt.Print(prompt + ":")
	password, err := terminal.ReadPassword(int(syscall.Stdin))
	CheckErr(err)
	fmt.Println("")
	return string(password)
}

func readKeyPassword() string {
	return readPasswordPrompt("Key password")
}

func appendChildToExecutablePath(child string) string {
	current, err := os.Executable()
	CheckErr(err)
	return filepath.Join(filepath.Dir(current), child)
}

func FileExists(clusterFile string) bool {
	_, err := os.Stat(clusterFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		panic(err)
	} else {
		return true
	}
}

func header(obj interface{}) {
	log.Print("*** ", obj)
}

func debug(prefix string, obj interface{}) {
	if deb {
		log.Print("--- "+prefix+":", obj)
	}
}

func logWithPrefix(prefix, str string) {
	log.Println(prefix + " : " + str)
}

func redirectLogs() *os.File {
	parent := filepath.Join(getCurrentDir(), "logs")
	err := os.MkdirAll(parent, os.ModePerm)
	CheckErr(err)
	f, err := os.OpenFile(filepath.Join(parent, "log.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	CheckErr(err)
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
	if err != nil {
		panic(string(bs))
	}
	return string(bs)
}

func execSSHCommandWithoutPanic(host, cmd string, config *ssh.ClientConfig) (string, error) {
	debug("SSH", host+", "+cmd)
	conn, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		debug("SSH:Dial failed", "")
		return "", err
	}
	session, err := conn.NewSession()
	if err != nil {
		debug("SSH:Session creation failed", "")
		return "", err
	}
	defer session.Close()
	bs, err := session.CombinedOutput(cmd)
	if err != nil {
		debug("SSH:session.CombinedOutput failed", "")
		return string(bs), err
	}
	return string(bs), nil
}

func getSSHSession(host string, config *ssh.ClientConfig) *ssh.Session {
	conn, err := ssh.Dial("tcp", host+":22", config)
	CheckErr(err)
	session, err := conn.NewSession()
	CheckErr(err)
	return session
}

func initSSHConnectionConfigWithPublicKeys(userName, privateKeyFile, password string) *ssh.ClientConfig {
	pemBytes, err := ioutil.ReadFile(privateKeyFile)
	CheckErr(err)
	signer, err := ssh.ParsePrivateKeyWithPassphrase(pemBytes, []byte(password))
	CheckErr(err)
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

func readFileIfExists(fileName, errorMessage string) []byte {
	nodesFileEntry, err := ioutil.ReadFile(filepath.Join(getCurrentDir(), fileName))
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal(errorMessage)
		}
	}
	return nodesFileEntry
}

func waitUserInput() string {
	buf := bufio.NewReader(os.Stdin)
	input, err := buf.ReadString('\n')
	CheckErr(err)
	return strings.Trim(input, "\n\r ")
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
		CheckErr(err)
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
	CheckErr(err)
	return convertExit
}

//CheckErr throws panic if error != nil
func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

func getNodesFromYml(parentFolderName string) []node {
	nodesFileName := filepath.Join(parentFolderName, nodesFileName)
	nodesFile, err := os.OpenFile(nodesFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	CheckErr(err)
	nodesFromYaml := make([]node, 0, 5)
	fileEntry, err := ioutil.ReadAll(nodesFile)
	CheckErr(err)
	err = yaml.Unmarshal(fileEntry, &nodesFromYaml)
	CheckErr(err)
	return nodesFromYaml
}

func numberHostsFromNodesFile(nodesFromYml []node) string {
	hostsWithNumbers := make(map[int]string, len(nodesFromYml))
	for i := range nodesFromYml {
		hostsWithNumbers[i] = nodesFromYml[i].Alias
		i++
	}
	log.Println("Please choose number of node from `nodesFileName`")
	return inputFuncForHosts(hostsWithNumbers)
}

func inputFuncForHosts(hostsWithNumbers map[int]string) string {
	b := new(bytes.Buffer)
	for k, v := range hostsWithNumbers {
		fmt.Printf("%d : %s\n", k, v)
	}
	log.Println("\n" + b.String())
	input := waitUserInput()
	convertedInput := convertStringToInt(input)
	if value, ok := hostsWithNumbers[convertedInput]; ok {
		return value
	}
	log.Println("Wrong number, specifys one of this!")
	return inputFuncForHosts(hostsWithNumbers)
}

func unmarshalClusterYml() *clusterFile {
	clusterFileEntry := readFileIfExists(swarmgoConfigFileName, "You should create swarmgo-config.yml")
	clusterFileStruct := clusterFile{}
	err := yaml.Unmarshal(clusterFileEntry, &clusterFileStruct)
	CheckErr(err)
	return &clusterFileStruct
}

func findSSHKeysAndInitConnection(passToKey string, config *clusterFile) *ssh.ClientConfig {
	_, privateKeyFile := findSSHKeys(config)
	if !FileExists(privateKeyFile) {
		log.Fatal("Can't find private key to connect to remote server!")
	}
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

func getCurrentDir() string {
	pwd, err := os.Getwd()
	CheckErr(err)
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
