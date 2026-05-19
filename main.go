package main

import (
	"os"

	"github.com/Hans-Zietsman/kunjani-cli/cmd"
)

var Version = "0.1.0"

func main() {
	root := cmd.NewRootCmd(Version)
	if err := root.Execute(); err != nil {
		os.Exit(cmd.ExitCode(err))
	}
}
