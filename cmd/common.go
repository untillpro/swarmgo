package cmd

import (
	"os"
	"path/filepath"
	"golang.org/x/crypto/ssh"
	"math/rand"
	"time"
	"fmt"
)

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

type ProductAndVersion struct {
	product, version string
}

func (pav ProductAndVersion) String() string {
	return fmt.Sprintf("%v=%v", pav.product, pav.version)
}

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

func execSshCommand(host, cmd string, config *ssh.ClientConfig) (string) {
	conn, err := ssh.Dial("tcp", host+":22", config)
	CheckErr(err)
	session, err := conn.NewSession()
	CheckErr(err)
	defer session.Close()

	bytes, err := session.CombinedOutput(cmd)

	if err != nil {
		panic(string(bytes))
	}
	return string(bytes)
}

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}
