package main

import (
	"fmt"
	"os"

	"github.com/rudrankriyam/Google-Health-CLI/cmd"
)

var (
	version = "1.0.0-dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	os.Exit(cmd.Run(os.Args[1:], fmt.Sprintf("%s (%s, %s)", version, commit, date)))
}
