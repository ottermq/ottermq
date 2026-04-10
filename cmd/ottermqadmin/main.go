package main

import (
	"fmt"
	"os"

	"github.com/ottermq/ottermq/internal/cli"
)

func main() {
	opts := &cli.RootOptions{}
	cmd := cli.NewRootCmd(opts)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
