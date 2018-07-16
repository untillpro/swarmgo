/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cmd

import (
	"github.com/spf13/cobra"
	"fmt"
)

// nodeCmd represents the node command
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Root command for nodes",
	Long:  `Root command for nodes: add`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Specify operation! \n" +
			"add <Alias1>=<IP1> <Alias2>=<IP2> ...")
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
}
