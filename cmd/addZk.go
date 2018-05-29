/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// addZkCmd represents the addZk command
var addZkCmd = &cobra.Command{
	Use:   "add",
	Short: "Not implemented yet",
	Long: `Not implemented yet`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not implemented yet")
	},
}

func init() {
	zkCmd.AddCommand(addZkCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// addZkCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// addZkCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
