package main

import (
	"fmt"
	"github.com/UFFeScience/akoflow/internal/montagetools"
	"os"
)

func main() {
	if err := montagetools.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
