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
	"io/ioutil"
	"log"
	"strings"
	"bufio"
	"github.com/mitchellh/go-homedir"
	"bytes"
	"fmt"
	"strconv"
	"gopkg.in/yaml.v2"
)

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
	bs, err := execSshCommandWithoutPanic(cmd, conn)
	if err != nil {
		panic(string(bs))
	}
	return string(bs)
}

func execSshCommandWithoutPanic(cmd string, conn *ssh.Client) (string, error) {
	session, err := conn.NewSession()
	CheckErr(err)
	defer session.Close()
	bs, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(bs), err
	}
	return string(bs), nil
}

func logWithPrefix(host, str string) {
	log.Println(host + " : " + str)
}

func redirectLogs() *os.File{
	parent := filepath.Join(getCurrentDir(),"logs")
	err := os.MkdirAll(parent, os.ModePerm)
	CheckErr(err)
	f, err := os.OpenFile(filepath.Join(parent,"log.log"), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	CheckErr(err)
	log.SetOutput(f)
	return f
}

func sudoExecSshCommand(host, cmd string, config *ssh.ClientConfig) string {
	return execSshCommand(host, "sudo "+cmd, config)
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

func contains(slice []string, find string) bool {
	for _, a := range slice {
		if a == find {
			return true
		}
	}
	return false
}

func containsNode(slice []Node, find Node) bool {
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

func findSshKeys(clusterName string) (string, string) {
	publicKeyName := ".ssh/" + clusterName + ".pub"
	privateKeyName := ".ssh/" + clusterName
	home, err := homedir.Dir()
	CheckErr(err)
	var publicKeyFile string
	var privateKeyFile string
	if len(home) > 0 {
		publicKeyFile = filepath.Join(home, publicKeyName)
		privateKeyFile = filepath.Join(home, privateKeyName)
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

func getNodesFromYml(parentFolderName string) []Node {
	nodesFileName := filepath.Join(parentFolderName, nodesFileName)
	nodesFile, err := os.OpenFile(nodesFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	CheckErr(err)
	nodesFromYaml := make([]Node, 0, 5)
	fileEntry, err := ioutil.ReadAll(nodesFile)
	CheckErr(err)
	err = yaml.Unmarshal(fileEntry, &nodesFromYaml)
	CheckErr(err)
	return nodesFromYaml
}

func numberHostsFromNodesFile(nodesFromYml []Node) string {
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
		fmt.Fprintf(b, "%d : %s\n", k, v)
	}
	log.Println("\n" + b.String())
	input := waitUserInput()
	convertedInput := convertStringToInt(input)
	if value, ok := hostsWithNumbers[convertedInput]; ok {
		return value
	} else {
		log.Println("Wrong number, specifys one of this!")
		return inputFuncForHosts(hostsWithNumbers)
	}
}

func unmarshalClusterYml() *ClusterFile {
	clusterFileEntry := readFileIfExists(clusterFileName, "Need to use swarmgo init first!")
	clusterFileStruct := ClusterFile{}
	err := yaml.Unmarshal(clusterFileEntry, &clusterFileStruct)
	CheckErr(err)
	return &clusterFileStruct
}

func findSshKeysAndInitConnection(clusterName, userName, passToKey string) *ssh.ClientConfig {
	_, privateKeyFile := findSshKeys(clusterName)
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
