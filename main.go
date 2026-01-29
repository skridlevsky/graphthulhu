package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/client"
)

func main() {
	lsClient := client.New("", "")

	srv := newServer(lsClient)

	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "graphthulhu: %v\n", err)
		os.Exit(1)
	}
}
