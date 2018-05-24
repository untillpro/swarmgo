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

func sudoExecSshCommand(host, cmd string, config *ssh.ClientConfig) string {
	return execSshCommand(host, "sudo " + cmd, config)
}

func initSshConnectionConfigWithPublicKeys(privateKeyFile string) *ssh.ClientConfig {
	pemBytes, err := ioutil.ReadFile(privateKeyFile)
	CheckErr(err)
	signer, err := ssh.ParsePrivateKey(pemBytes)
	CheckErr(err)
	log.Println("Connecting to remote servers with public key")
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

func readClusterfileIfExists() []byte{
	clusterFileEntry, err := ioutil.ReadFile(appendChildToExecutablePath(clusterFileName))
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Need to use `swarmgo init` first!")
		}
	}
	return clusterFileEntry
}

func readNodesFileIfExists() []byte{
	nodesFileEntry, err := ioutil.ReadFile(appendChildToExecutablePath(nodes))
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatal("Need to add some nodes first!")
		}
	}
	return nodesFileEntry
}

func waitUserInput() string {
	buf := bufio.NewReader(os.Stdin)
	input, err := buf.ReadString('\n')
	CheckErr(err)
	return input
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

func takeHostFromFlagOrChooseFromNodesFile(nodesFileEntry []byte) string{
	re := regexp.MustCompile(`\r?\n`)
	input := re.ReplaceAllString(string(nodesFileEntry), " ")
	hosts := strings.Split(input, " ")
	var host string
	if len(ip) > 0 {
		if contains(hosts, ip) {
			host = ip
		} else {
			log.Fatal("--ip=<value> doesn't present in `nodes`!")
		}
	} else {
		hostsWithNumbers := make(map[int]string, len(hosts))
		for i := range hosts {
			hostsWithNumbers[i] = hosts[i]
			i++
		}
		log.Println("Please choose number of node from `nodes`")
		host = inputFunc(hostsWithNumbers)
	}
	return host
}

func inputFunc(hostsWithNumbers map[int]string) string {
	b := new(bytes.Buffer)
	for k, v := range hostsWithNumbers {
		fmt.Fprintf(b, "%d : %s\n", k, v)
	}
	log.Println("\n" + b.String())
	input := strings.Trim(waitUserInput(), "\n ")
	convertedInput, err := strconv.Atoi(input)
	CheckErr(err)
	if value, ok := hostsWithNumbers[convertedInput]; ok {
		return value
	} else {
		log.Println("Wrong number, specifys one of this!")
		return inputFunc(hostsWithNumbers)
	}
}

func findDockerVersionFromClusterfile() string{
	clusterFileEntry := readClusterfileIfExists()
	productsAndVersions := parseFileBytesAsMap(clusterFileEntry)
	version, ok := productsAndVersions[docker]
	if !ok {
		//TODO maybe install latest?
		log.Fatal("Can't find docker version from Clusterfile!")
	}
	return version
}

func findSshKeysAndInitConnection() *ssh.ClientConfig {
	_, privateKeyFile := findSshKeys()
	if !checkFileExistence(privateKeyFile) {
		log.Fatal("Can't find private key to connect to remote server!")
	}
	return initSshConnectionConfigWithPublicKeys(privateKeyFile)
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
