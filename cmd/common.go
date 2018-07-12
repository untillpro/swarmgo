/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cmd

import (
	"os"
	"path/filepath"
	"golang.org/x/crypto/ssh"
	"math/rand"
	"time"
	"io/ioutil"
	"log"
	"strings"
	"bufio"
	"github.com/mitchellh/go-homedir"
	"regexp"
	"bytes"
	"fmt"
	"strconv"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func generateRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func appendChildToExecutablePath(child string) string {
	current, err := os.Executable()
	CheckErr(err)
	return filepath.Join(filepath.Dir(current), child)
}

func checkFileExistence(clusterFile string) bool {
	_, err := os.Stat(clusterFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		} else {
			panic(err)
		}
	} else {
		return true
	}
}

func execSshCommand(host, cmd string, config *ssh.ClientConfig) string {
	conn, err := ssh.Dial("tcp", host+":22", config)
	CheckErr(err)
	session, err := conn.NewSession()
	CheckErr(err)
	defer session.Close()

	bs, err := session.CombinedOutput(cmd)

	if err != nil {
		panic(string(bs))
	}
	return string(bs)
}

func logWithPrefix(host, str string) {
	log.Println(host + " : " + str)
}

func sudoExecSshCommand(host, cmd string, config *ssh.ClientConfig) string {
	return execSshCommand(host, "sudo " + cmd, config)
}

func initSshConnectionConfigWithPublicKeys(userName, privateKeyFile, password string) *ssh.ClientConfig {
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

func parseFileBytesAsMap(file []byte) map[string]string {
	pav := strings.Split(string(file), "\n")
	if len(pav) == 1 && pav[0] == "" {
		return make(map[string]string, 1)
	}
	structs := make(map[string]string, len(pav))
	for i := range pav {
		arr := strings.Split(pav[i], "=")
		structs[arr[0]] = arr[1]
	}
	return structs
}

func contains(slice []string, find string) bool {
	for _, a := range slice {
		if a == find {
			return true
		}
	}
	return false
}

func readFileIfExists(fileName, errorMessage string) []byte{
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

func findSshKeys() (string, string) {
	publicKeyName := ".ssh/id_rsa.pub"
	privateKeyName := ".ssh/id_rsa"
	home, err := homedir.Dir()
	CheckErr(err)
	var publicKeyFile string
	var privateKeyFile string
	if len(home) > 0 {
		publicKeyFile = filepath.Join(home, ".ssh/id_rsa.pub")
		privateKeyFile = filepath.Join(home, ".ssh/id_rsa")
	} else {
		publicKeyFile = appendChildToExecutablePath(publicKeyName)
		privateKeyFile = appendChildToExecutablePath(privateKeyName)
	}
	return publicKeyFile, privateKeyFile
}

func checkDockerInstallation(host, version string, config *ssh.ClientConfig) bool {
	defer func() {
		//just catching stderr from ubuntu because `docker` is unknown command
		recover()
	}()
	exit := execSshCommand(host, "docker -v", config)
	trimmedVersion := strings.Split(version, "~")[0]
	return strings.Contains(exit, trimmedVersion)
}

func checkSwarmExistence(host string, config *ssh.ClientConfig) bool{
	defer func() {
		//just catching stderr from ubuntu because `docker` is unknown command
		recover()
	}()
	exit := execSshCommand(host, "docker node ls | echo $?", config)
	converted := convertStringToInt(exit)
	return converted == 0
}

func convertStringToInt(s string) int {
	convertExit, err := strconv.Atoi(strings.TrimSuffix(s, "\n"))
	CheckErr(err)
	return convertExit
}

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

func takeHostsFromArgsOrChooseFromNodesFile(nodesFileEntry []byte, args []string) []string{
	re := regexp.MustCompile(`\r?\n`)
	input := re.ReplaceAllString(string(nodesFileEntry), " ")
	knownHosts := strings.Split(strings.Trim(input, " "), " ")
	var hosts []string
	if len(args) > 0 {
		hosts = make([]string, len(args))
		for i :=range args {
			if contains(knownHosts, args[i]) {
				hosts[i] = args[i]
			} else {
				log.Fatal("--ip=<value> doesn't present in `nodes`!")
			}
		}
	} else {
		hosts = make([]string, 1)
		hosts[0] = numberHostsFromNodesFile(knownHosts)
	}
	return hosts
}

func numberHostsFromNodesFile(knownHosts []string) string{
	hostsWithNumbers := make(map[int]string, len(knownHosts))
	for i := range knownHosts {
		hostsWithNumbers[i] = knownHosts[i]
		i++
	}
	log.Println("Please choose number of node from `nodes`")
	return inputFuncForHosts(hostsWithNumbers)
}

func inputFuncForHosts(hostsWithNumbers map[int]string) string {
	b := new(bytes.Buffer)
	for k, v := range hostsWithNumbers {
		fmt.Fprintf(b, "%d : %s\n", k, v)
	}
	log.Println("\n" + b.String())
	input := waitUserInput()
	convertedInput, err := strconv.Atoi(input)
	CheckErr(err)
	if value, ok := hostsWithNumbers[convertedInput]; ok {
		return value
	} else {
		log.Println("Wrong number, specifys one of this!")
		return inputFuncForHosts(hostsWithNumbers)
	}
}

func findDockerVersionFromClusterfile() string{
	clusterFileEntry := readFileIfExists(clusterFileName, "Need to use swarmgo init first!")
	productsAndVersions := parseFileBytesAsMap(clusterFileEntry)
	version, ok := productsAndVersions[docker]
	if !ok {
		//TODO maybe install latest?
		log.Fatal("Can't find docker version from Clusterfile!")
	}
	return version
}

func findSshKeysAndInitConnection(userName, passToKey string) *ssh.ClientConfig {
	_, privateKeyFile := findSshKeys()
	if !checkFileExistence(privateKeyFile) {
		log.Fatal("Can't find private key to connect to remote server!")
	}
	return initSshConnectionConfigWithPublicKeys(userName, privateKeyFile, passToKey)
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

func getCurrentDir() string {
	pwd, err := os.Getwd()
	CheckErr(err)
	return pwd
}