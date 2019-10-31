/*
 * Copyright (c) 2019-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

var logFile *os.File

type loggedRunnable func(cmd *cobra.Command, args []string)

func loggedCmd(f loggedRunnable) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		initCommand(cmd.Name())
		defer finitCommand()
		f(cmd, args)
	}
}

func initCommand(cmdName string) {
	gc.IsVerbose = true
	gc.Output = myOutput

	n := time.Now()
	logFileName := n.Format("20060102-150405-") + cmdName + ".txt"
	logFolderName := filepath.Join(getWorkingDir(), "logs")
	gc.ExitIfError(os.MkdirAll(logFolderName, os.ModePerm), "Could not not create a folder for logs")

	logFilePath := filepath.Join(logFolderName, logFileName)

	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	gc.ExitIfError(err, "Could not create a log file "+logFilePath)

}

func finitCommand() {
	if nil != logFile {
		logFile.Close()
		logFile = nil
	}
}

func myOutput(funcName, s string) {
	if "Verbose" != funcName || verbose {
		fmt.Print(s)
	}
	n := time.Now()
	line := n.Format("20060102 15:04:05.000 ") + s
	fmt.Fprint(logFile, line)
}
