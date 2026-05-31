package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	ilnmcp "github.com/beabys/ilnamiqui/internal/mcp"
	"github.com/beabys/ilnamiqui/internal/service"
)

func main() {
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	defer svc.Close()

	s := server.NewMCPServer(
		"ilnamiqui-mcp",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	handler := ilnmcp.NewHandler(svc)
	handler.RegisterTools(s)

	stdio := server.NewStdioServer(s)
	fmt.Fprintf(os.Stderr, "ilnamiqui-mcp: starting stdio server\n")
	if err := stdio.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "ilnamiqui-mcp: error: %v\n", err)
		os.Exit(1)
	}
}
