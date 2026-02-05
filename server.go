package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/backend"
	"github.com/skridlevsky/graphthulhu/tools"
	"github.com/skridlevsky/graphthulhu/vault"
)

// newServer creates and configures the MCP server with all tools registered.
// If readOnly is true, write tools are not registered.
// Tools requiring DataScript are only registered if the backend supports it.
func newServer(b backend.Backend, readOnly bool) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    "graphthulhu",
			Version: version,
		},
		nil,
	)

	_, hasDataScript := b.(backend.HasDataScript)

	nav := tools.NewNavigate(b)
	search := tools.NewSearch(b)
	analyze := tools.NewAnalyze(b)
	journal := tools.NewJournal(b)

	var write *tools.Write
	var decision *tools.Decision
	if !readOnly {
		write = tools.NewWrite(b)
		decision = tools.NewDecision(b)
	}

	// --- Navigate tools (all backends) ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_page",
		Description: "Get a Logseq page with its full recursive block tree, properties, tags, and parsed links. Every block includes extracted [[links]], ((references)), #tags, and key:: value properties. Use maxBlocks to limit output size for large pages.",
	}, nav.GetPage)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_block",
		Description: "Get a specific block by UUID with its ancestor chain (path to root page), children, and optionally siblings. Provides full context for where a block sits in the knowledge hierarchy.",
	}, nav.GetBlock)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_pages",
		Description: "List pages with filtering by namespace, property, or tag. Returns page summaries with block count and link count. Sort by name, modified, or created.",
	}, nav.ListPages)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_links",
		Description: "Get forward links (pages this page links to) and backlinks (pages that link to this page) for any page. Each link includes the specific block containing it.",
	}, nav.GetLinks)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "traverse",
		Description: "Find paths between two pages through the link graph using BFS. Discovers how concepts are connected through intermediate pages. Returns all paths up to max_hops length.",
	}, nav.Traverse)

	// get_references requires DataScript for ancestor lookups.
	if hasDataScript {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "get_references",
			Description: "Get all blocks that reference a specific block via ((uuid)) block references. Returns the referencing blocks with their page context.",
		}, nav.GetReferences)
	}

	// --- Search tools ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search",
		Description: "Full-text search across all blocks in the knowledge graph. Returns matching blocks with surrounding context (parent chain and sibling blocks) so you understand where each match sits.",
	}, search.Search)

	// query_properties and find_by_tag use native search on Obsidian, DataScript on Logseq.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_properties",
		Description: "Find blocks and pages by property values. Search for all content with a specific property key, or filter by property value with operators (eq, contains, gt, lt).",
	}, search.QueryProperties)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "find_by_tag",
		Description: "Find all blocks and pages with a specific tag, including child tags in the tag hierarchy. Returns content grouped by page.",
	}, search.FindByTag)

	// query_datalog is inherently Logseq-specific.
	if hasDataScript {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "query_datalog",
			Description: "Execute raw DataScript/Datalog queries against the Logseq database. This is the most powerful query mechanism — can find anything. Example: [:find (pull ?b [*]) :where [?b :block/marker \"TODO\"]] finds all TODO blocks.",
		}, search.QueryDatalog)
	}

	// --- Analyze tools (all backends — use graph.Build which only needs GetAllPages + GetPageBlocksTree) ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "graph_overview",
		Description: "Get a high-level overview of the entire knowledge graph: total pages, blocks, links, most connected pages, orphan count, namespace breakdown. Builds an in-memory graph for analysis.",
	}, analyze.GraphOverview)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "find_connections",
		Description: "Discover how two pages are connected through the link graph. Returns whether they're directly linked, shortest paths between them, and shared connections (pages both link to or are linked from).",
	}, analyze.FindConnections)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "knowledge_gaps",
		Description: "Find sparse areas in the knowledge graph: orphan pages (no links in or out), dead-end pages (linked to but link nowhere), and weakly-linked pages. Helps identify where to add connections.",
	}, analyze.KnowledgeGaps)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_orphans",
		Description: "List orphan pages (no incoming or outgoing links). Returns page names with block counts and property status. Use for graph hygiene — find disconnected pages that need linking or cleanup.",
	}, analyze.ListOrphans)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "topic_clusters",
		Description: "Discover topic clusters by finding connected components in the knowledge graph. Returns groups of densely connected pages with their hub (most connected page in each cluster).",
	}, analyze.TopicClusters)

	// --- Write tools (skipped in read-only mode) ---
	if !readOnly {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "create_page",
			Description: "Create a new Logseq page with optional properties and initial blocks. Use properties for metadata like type::, status::, etc.",
		}, write.CreatePage)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "append_blocks",
			Description: "Append plain-text blocks to an existing page. Accepts an array of strings (same format as create_page blocks). Simpler than upsert_blocks when you just need to add content without nesting or properties.",
		}, write.AppendBlocks)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "update_block",
			Description: "Update an existing block's content by UUID. Replaces the block's entire content with the new value. Use get_page or get_block first to find the UUID.",
		}, write.UpdateBlock)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "delete_block",
			Description: "Delete a block by UUID. Removes the block and all its children from the graph. This is irreversible.",
		}, write.DeleteBlock)

		// upsert_blocks uses raw handler because BlockInput has recursive Children field
		// which the schema generator can't handle.
		srv.AddTool(&mcp.Tool{
			Name:        "upsert_blocks",
			Description: "Batch create blocks on a page. Supports nested children for building block hierarchies. Append or prepend to existing content.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"page":{"type":"string","description":"Page name to add blocks to"},"blocks":{"type":"array","description":"Blocks to create. Each block has content (string), optional properties (object of strings), and optional children (array of blocks).","items":{"type":"object"}},"position":{"type":"string","description":"Where to add: append or prepend. Default: append"}},"required":["page","blocks"],"additionalProperties":false}`),
		}, write.UpsertBlocksRaw)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "move_block",
			Description: "Move a block to a new location — before, after, or as a child of another block.",
		}, write.MoveBlock)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "delete_page",
			Description: "Delete a page entirely from the graph. Removes the page and all its blocks. This is irreversible.",
		}, write.DeletePage)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "rename_page",
			Description: "Rename a page and update all [[links]] across the graph that reference the old name. Preserves content and connections.",
		}, write.RenamePage)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "bulk_update_properties",
			Description: "Set a property (key:: value) on multiple pages at once. Useful for backfilling metadata like type::, status::, etc. across many pages in one call.",
		}, write.BulkUpdateProperties)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "link_pages",
			Description: "Create a bidirectional connection between two pages by adding a link block to each. Optionally include context describing the relationship.",
		}, write.LinkPages)
	}

	// --- Decision tools (skipped in read-only mode) ---
	if !readOnly {
		mcp.AddTool(srv, &mcp.Tool{
			Name:        "decision_check",
			Description: "Surface all tracked decisions in the knowledge graph. Returns open, overdue, and optionally resolved decisions with deadline status. Use at session start to check what needs attention.",
		}, decision.DecisionCheck)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "decision_create",
			Description: "Create a new DECIDE block on a page with #decision tag and deadline. Decisions live on the page where context is richest, not on a central backlog.",
		}, decision.DecisionCreate)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "decision_resolve",
			Description: "Mark a decision as DONE with today's date and an optional outcome. Changes the DECIDE marker to DONE and adds resolved:: property.",
		}, decision.DecisionResolve)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "decision_defer",
			Description: "Push a decision's deadline to a new date with a reason. Tracks deferral count and warns after 3+ deferrals.",
		}, decision.DecisionDefer)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "analysis_health",
			Description: "Audit analysis, strategy, and assessment pages for graph connectivity. A page is healthy if it has 3+ outgoing links or contains a decision. Finds isolated analyses that don't connect back to the knowledge graph.",
		}, decision.AnalysisHealth)
	}

	// --- Journal tools (all backends — search uses native fallback for non-DataScript) ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "journal_range",
		Description: "Get journal entries across a date range. Returns journal pages with their full block trees. Dates in YYYY-MM-DD format.",
	}, journal.JournalRange)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "journal_search",
		Description: "Search within journal entries specifically. Optionally filter by date range. Returns matching blocks with their journal date context.",
	}, journal.JournalSearch)

	// --- Flashcard tools (DataScript-only for overview/due) ---
	if hasDataScript {
		flashcard := tools.NewFlashcard(b)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "flashcard_overview",
			Description: "Get SRS (spaced repetition) statistics: total cards, cards due for review, new vs reviewed cards, average repetitions. Gives a snapshot of the flashcard collection health.",
		}, flashcard.FlashcardOverview)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "flashcard_due",
			Description: "Get flashcards currently due for review. Returns card content, page, and SRS properties (ease factor, interval, repeats). Prioritizes new cards and overdue reviews.",
		}, flashcard.FlashcardDue)

		if !readOnly {
			mcp.AddTool(srv, &mcp.Tool{
				Name:        "flashcard_create",
				Description: "Create a new flashcard on a page. Adds a block with #card tag (front/question) and a child block (back/answer). The card will appear in Logseq's flashcard review system.",
			}, flashcard.FlashcardCreate)
		}
	}

	// --- Whiteboard tools (Logseq-specific) ---
	if hasDataScript {
		whiteboard := tools.NewWhiteboard(b)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "list_whiteboards",
			Description: "List all Logseq whiteboards in the graph. Whiteboards are infinite canvas spaces where concepts are visually arranged and connected.",
		}, whiteboard.ListWhiteboards)

		mcp.AddTool(srv, &mcp.Tool{
			Name:        "get_whiteboard",
			Description: "Get a whiteboard's content including embedded pages, block references, visual connections between elements, and any text content. Reveals how concepts are spatially organized.",
		}, whiteboard.GetWhiteboard)
	}

	// --- Health tool (all backends) ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "health",
		Description: "Check server status: version, backend type, read-only mode, page count. Use to verify the server is alive and see its configuration.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, any, error) {
		backendType := "logseq"
		if _, ok := b.(backend.HasDataScript); !ok {
			backendType = "obsidian"
		}

		pages, _ := b.GetAllPages(ctx)
		pingErr := b.Ping(ctx)

		status := "ok"
		if pingErr != nil {
			status = fmt.Sprintf("error: %v", pingErr)
		}

		data, _ := json.MarshalIndent(map[string]any{
			"status":    status,
			"version":   version,
			"backend":   backendType,
			"readOnly":  readOnly,
			"pageCount": len(pages),
		}, "", "  ")

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		}, nil, nil
	})

	// --- Vault management tools (Obsidian-specific) ---
	if vaultClient, ok := b.(*vault.Client); ok && !readOnly {
		srv.AddTool(&mcp.Tool{
			Name:        "reload",
			Description: "Force a full vault re-index. Clears all cached pages and blocks, then re-reads all .md files. Use when external changes need to be refreshed.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{},"required":[],"additionalProperties":false}`),
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if err := vaultClient.Reload(); err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Reload failed: %v", err)}},
					IsError: true,
				}, nil
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "Vault reloaded successfully"}},
			}, nil
		})
	}

	return srv
}
