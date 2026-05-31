package main

import (
	"fmt"
	"os"

	"github.com/beabys/ilnamiqui/internal/cli"
	"github.com/beabys/ilnamiqui/internal/service"
)

func main() {
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	defer svc.Close() //nolint:errcheck
	if err := cli.New(svc).Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
