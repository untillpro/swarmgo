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

var addZkCmd = &cobra.Command{
	Use:   "add",
	Short: "Not implemented yet",
	Long:  `Not implemented yet`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Not implemented yet")
	},
}

func init() {
	zkCmd.AddCommand(addZkCmd)
}
