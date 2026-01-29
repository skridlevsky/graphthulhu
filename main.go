package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/client"
)

var version = "dev"

func main() {
	readOnly := flag.Bool("read-only", false, "Disable all write operations")
	flag.Parse()

	lsClient := client.New("", "")

	checkGraphVersionControl(lsClient)

	srv := newServer(lsClient, *readOnly)

	if err := srv.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "graphthulhu: %v\n", err)
		os.Exit(1)
	}
}

// checkGraphVersionControl warns on stderr if the Logseq graph is not git-controlled.
// Best-effort: silently skips if Logseq is not running or the API is unreachable.
func checkGraphVersionControl(c *client.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	graph, err := c.GetCurrentGraph(ctx)
	if err != nil || graph == nil || graph.Path == "" {
		return
	}

	gitDir := filepath.Join(graph.Path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "graphthulhu: WARNING: graph %q at %s is not version controlled\n", graph.Name, graph.Path)
		fmt.Fprintf(os.Stderr, "graphthulhu: Write operations cannot be undone. Consider: cd %s && git init && git add -A && git commit -m 'initial'\n", graph.Path)
	}
}
