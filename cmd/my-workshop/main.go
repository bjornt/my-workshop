package main

import (
	"fmt"
	"os"

	"github.com/bjornt/my-workshop/internal/cli"
	"github.com/bjornt/my-workshop/internal/workshop"
)

func main() {
	log := workshop.DefaultLogger
	if err := cli.Run(os.Args[1:], nil, log); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
