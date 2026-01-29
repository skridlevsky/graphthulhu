package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/skridlevsky/graphthulhu/client"
	"github.com/skridlevsky/graphthulhu/types"
)

// runJournal appends a block to today's (or a specified date's) journal page.
func runJournal(args []string, c *client.Client) {
	fs := flag.NewFlagSet("journal", flag.ExitOnError)
	date := fs.String("date", "", "Journal date (YYYY-MM-DD). Default: today")
	fs.StringVar(date, "d", "", "Journal date (YYYY-MM-DD). Default: today")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: graphthulhu journal [--date YYYY-MM-DD] CONTENT\n")
		fmt.Fprintf(os.Stderr, "       echo CONTENT | graphthulhu journal\n\n")
		fmt.Fprintf(os.Stderr, "Appends a block to a Logseq journal page.\n")
		fmt.Fprintf(os.Stderr, "Prints the created block UUID on success.\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	content := readContent(fs)
	if content == "" {
		fs.Usage()
		os.Exit(1)
	}

	var t time.Time
	if *date != "" {
		var err error
		t, err = time.Parse("2006-01-02", *date)
		if err != nil {
			fmt.Fprintf(os.Stderr, "graphthulhu journal: invalid date %q (use YYYY-MM-DD)\n", *date)
			os.Exit(1)
		}
	} else {
		t = time.Now()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pageName := findJournalPage(ctx, c, t)
	if pageName == "" {
		// No existing page found — use ordinal format (most common Logseq default).
		pageName = ordinalDate(t)
	}

	block, err := c.AppendBlockInPage(ctx, pageName, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "graphthulhu journal: %v\n", err)
		os.Exit(1)
	}

	if block != nil {
		fmt.Println(block.UUID)
	}
}

// runAdd appends a block to a named page.
func runAdd(args []string, c *client.Client) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	page := fs.String("page", "", "Page name (required)")
	fs.StringVar(page, "p", "", "Page name (required)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: graphthulhu add --page PAGE CONTENT\n")
		fmt.Fprintf(os.Stderr, "       echo CONTENT | graphthulhu add -p PAGE\n\n")
		fmt.Fprintf(os.Stderr, "Appends a block to a Logseq page (creates page if needed).\n")
		fmt.Fprintf(os.Stderr, "Prints the created block UUID on success.\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if *page == "" {
		fmt.Fprintf(os.Stderr, "graphthulhu add: --page is required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	content := readContent(fs)
	if content == "" {
		fmt.Fprintf(os.Stderr, "graphthulhu add: no content provided\n\n")
		fs.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	block, err := c.AppendBlockInPage(ctx, *page, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "graphthulhu add: %v\n", err)
		os.Exit(1)
	}

	if block != nil {
		fmt.Println(block.UUID)
	}
}

// runSearch performs full-text search and prints results to stdout.
func runSearch(args []string, c *client.Client) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	limit := fs.Int("limit", 10, "Max results")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: graphthulhu search [-limit N] QUERY\n\n")
		fmt.Fprintf(os.Stderr, "Full-text search across all blocks in the knowledge graph.\n\n")
		fs.PrintDefaults()
	}
	fs.Parse(args)

	query := strings.Join(fs.Args(), " ")
	if query == "" {
		fs.Usage()
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queryLower := strings.ToLower(query)
	pages, err := c.GetAllPages(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "graphthulhu search: %v\n", err)
		os.Exit(1)
	}

	found := 0
	for _, page := range pages {
		if found >= *limit {
			break
		}
		if page.Name == "" {
			continue
		}

		blocks, err := c.GetPageBlocksTree(ctx, page.Name)
		if err != nil {
			continue
		}

		printSearchResults(blocks, queryLower, page.OriginalName, *limit, &found)
	}

	if found == 0 {
		fmt.Fprintf(os.Stderr, "no results for %q\n", query)
		os.Exit(1)
	}
}

// --- Helpers ---

// readContent gets content from positional args or stdin (if piped).
func readContent(fs *flag.FlagSet) string {
	if args := fs.Args(); len(args) > 0 {
		return strings.Join(args, " ")
	}

	// Only read stdin if it's piped (not a terminal).
	stat, err := os.Stdin.Stat()
	if err != nil {
		return ""
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "" // stdin is a terminal, not piped
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// findJournalPage tries common Logseq journal date formats to find an existing page.
func findJournalPage(ctx context.Context, c *client.Client, t time.Time) string {
	names := []string{
		ordinalDate(t),
		t.Format("2006-01-02"),
		t.Format("January 2, 2006"),
	}

	for _, name := range names {
		page, err := c.GetPage(ctx, name)
		if err == nil && page != nil {
			return name
		}
	}
	return ""
}

// ordinalDate formats a time as "Jan 29th, 2026" (common Logseq journal default).
func ordinalDate(t time.Time) string {
	day := t.Day()
	suffix := "th"
	switch {
	case day == 1 || day == 21 || day == 31:
		suffix = "st"
	case day == 2 || day == 22:
		suffix = "nd"
	case day == 3 || day == 23:
		suffix = "rd"
	}
	return fmt.Sprintf("%s %d%s, %d", t.Format("Jan"), day, suffix, t.Year())
}

// printSearchResults recursively prints matching blocks to stdout.
func printSearchResults(blocks []types.BlockEntity, query, pageName string, limit int, found *int) {
	for _, b := range blocks {
		if *found >= limit {
			return
		}
		if strings.Contains(strings.ToLower(b.Content), query) {
			fmt.Printf("%s | %s\n", pageName, b.Content)
			*found++
		}
		if len(b.Children) > 0 {
			printSearchResults(b.Children, query, pageName, limit, found)
		}
	}
}
