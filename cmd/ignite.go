/*
Copyright © 2021 Opsani <support@opsani.com>
This file is part of https://github.com/opsani/opsani-ignite
*/

package cmd

import (
	"fmt"
	"os"
	"github.com/spf13/cobra"
)

func run_ignite(cmd *cobra.Command, args []string) { 
	fmt.Printf("Getting Prometheus metrics from %q\n", promUri)

	x, err := promGetAll()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("awesome:", x) 
}