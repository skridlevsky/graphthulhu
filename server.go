package main

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/client"
	"github.com/skridlevsky/graphthulhu/tools"
)

// newServer creates and configures the MCP server with all tools registered.
// If readOnly is true, write tools are not registered.
func newServer(lsClient *client.Client, readOnly bool) *mcp.Server {
	srv := mcp.NewServer(
		&mcp.Implementation{
			Name:    "graphthulhu",
			Version: "0.1.0",
		},
		nil,
	)

	nav := tools.NewNavigate(lsClient)
	search := tools.NewSearch(lsClient)
	analyze := tools.NewAnalyze(lsClient)
	journal := tools.NewJournal(lsClient)
	flashcard := tools.NewFlashcard(lsClient)
	whiteboard := tools.NewWhiteboard(lsClient)

	var write *tools.Write
	if !readOnly {
		write = tools.NewWrite(lsClient)
	}

	// --- Navigate tools ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_page",
		Description: "Get a Logseq page with its full recursive block tree, properties, tags, and parsed links. Every block includes extracted [[links]], ((references)), #tags, and key:: value properties.",
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
		Name:        "get_references",
		Description: "Get all blocks that reference a specific block via ((uuid)) block references. Returns the referencing blocks with their page context.",
	}, nav.GetReferences)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "traverse",
		Description: "Find paths between two pages through the link graph using BFS. Discovers how concepts are connected through intermediate pages. Returns all paths up to max_hops length.",
	}, nav.Traverse)

	// --- Search tools ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "search",
		Description: "Full-text search across all blocks in the knowledge graph. Returns matching blocks with surrounding context (parent chain and sibling blocks) so you understand where each match sits.",
	}, search.Search)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_properties",
		Description: "Find blocks and pages by property values. Search for all content with a specific property key, or filter by property value with operators (eq, contains, gt, lt).",
	}, search.QueryProperties)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_datalog",
		Description: "Execute raw DataScript/Datalog queries against the Logseq database. This is the most powerful query mechanism — can find anything. Example: [:find (pull ?b [*]) :where [?b :block/marker \"TODO\"]] finds all TODO blocks.",
	}, search.QueryDatalog)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "find_by_tag",
		Description: "Find all blocks and pages with a specific tag, including child tags in the tag hierarchy. Returns content grouped by page.",
	}, search.FindByTag)

	// --- Analyze tools ---
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
			Name:        "link_pages",
			Description: "Create a bidirectional connection between two pages by adding a link block to each. Optionally include context describing the relationship.",
		}, write.LinkPages)
	}

	// --- Journal tools ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "journal_range",
		Description: "Get journal entries across a date range. Returns journal pages with their full block trees. Dates in YYYY-MM-DD format.",
	}, journal.JournalRange)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "journal_search",
		Description: "Search within journal entries specifically. Optionally filter by date range. Returns matching blocks with their journal date context.",
	}, journal.JournalSearch)

	// --- Flashcard tools ---
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

	// --- Whiteboard tools ---
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_whiteboards",
		Description: "List all Logseq whiteboards in the graph. Whiteboards are infinite canvas spaces where concepts are visually arranged and connected.",
	}, whiteboard.ListWhiteboards)

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_whiteboard",
		Description: "Get a whiteboard's content including embedded pages, block references, visual connections between elements, and any text content. Reveals how concepts are spatially organized.",
	}, whiteboard.GetWhiteboard)

	return srv
}
