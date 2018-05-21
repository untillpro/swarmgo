package cmd

import (
	"os"
	"path/filepath"
)

func appendChildToExecutablePath(child string) string {
	current, err := os.Executable()
	if err != nil {
		panic(err)
	}
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
