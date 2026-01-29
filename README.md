# graphthulhu

MCP server that gives AI full read-write access to your Logseq knowledge graph. Navigate pages, search blocks, analyze link structure, manage flashcards, and write content — all through 27 tools over the [Model Context Protocol](https://modelcontextprotocol.io).

Built in Go with the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

## Why

Logseq stores knowledge as interconnected pages, blocks, and links. But AI assistants can't see any of it — they're blind to your second brain.

graphthulhu fixes that. It exposes your entire knowledge graph through MCP, so Claude (or any MCP client) can:

- Read any page with its full block tree, parsed links, tags, and properties
- Search across all blocks with contextual results (parent chain + siblings)
- Traverse the link graph to discover how concepts connect
- Find knowledge gaps — orphan pages, dead ends, weakly-linked areas
- Discover topic clusters through connected component analysis
- Create pages, write blocks, build hierarchies, link pages bidirectionally
- Query with raw DataScript/Datalog for anything the built-in tools don't cover
- Review flashcards with spaced repetition statistics
- Explore whiteboards and their spatial connections

It turns "tell me about X" into an AI that actually understands your knowledge graph's structure.

## Tools

27 tools across 7 categories:

### Navigate (6 tools)

| Tool | Description |
|------|-------------|
| `get_page` | Full recursive block tree with parsed links, tags, properties |
| `get_block` | Block by UUID with ancestor chain, children, siblings |
| `list_pages` | Filter by namespace, property, or tag; sort by name/modified/created |
| `get_links` | Forward and backward links with the blocks that contain them |
| `get_references` | All blocks referencing a specific block via `((uuid))` |
| `traverse` | BFS path-finding between two pages through the link graph |

### Search (4 tools)

| Tool | Description |
|------|-------------|
| `search` | Full-text search with parent chain + sibling context |
| `query_properties` | Find by property values with operators (eq, contains, gt, lt) |
| `query_datalog` | Raw DataScript/Datalog queries against the Logseq database |
| `find_by_tag` | Tag search with child tag hierarchy support |

### Analyze (4 tools)

| Tool | Description |
|------|-------------|
| `graph_overview` | Global stats: pages, blocks, links, most connected, namespaces |
| `find_connections` | Direct links, shortest paths, shared connections between pages |
| `knowledge_gaps` | Orphan pages, dead ends, weakly-linked areas |
| `topic_clusters` | Connected components with hub identification |

### Write (6 tools)

| Tool | Description |
|------|-------------|
| `create_page` | New page with properties and initial blocks |
| `upsert_blocks` | Batch create with nested children for deep hierarchies |
| `update_block` | Replace block content by UUID |
| `delete_block` | Remove block and all children |
| `move_block` | Reposition before, after, or as child of another block |
| `link_pages` | Bidirectional link with optional relationship context |

### Journal (2 tools)

| Tool | Description |
|------|-------------|
| `journal_range` | Entries across a date range with full block trees |
| `journal_search` | Search within journals, optionally filtered by date |

### Flashcard (3 tools)

| Tool | Description |
|------|-------------|
| `flashcard_overview` | SRS stats: total, due, new vs reviewed, average repeats |
| `flashcard_due` | Cards due for review with ease factor and interval |
| `flashcard_create` | Create front/back card with `#card` tag |

### Whiteboard (2 tools)

| Tool | Description |
|------|-------------|
| `list_whiteboards` | All whiteboards in the graph |
| `get_whiteboard` | Embedded pages, block references, visual connections |

## Install

### Download binary

Grab the latest release for your platform from [GitHub Releases](https://github.com/skridlevsky/graphthulhu/releases) and add it to your PATH.

### go install

```bash
go install github.com/skridlevsky/graphthulhu@latest
```

### Build from source

```bash
git clone https://github.com/skridlevsky/graphthulhu.git
cd graphthulhu
go build -o graphthulhu .
```

### Enable Logseq HTTP API

1. In Logseq, go to **Settings → Features** and enable **HTTP APIs server**
2. Click the **API** icon that appears in the top toolbar
3. Click **Start Server**
4. Click **Create Token** and copy the generated token — you'll need it for configuration

The API runs on `http://127.0.0.1:12315` by default.

## Configuration

### Claude Code

Add to your MCP settings (`~/.claude/claude_code_config.json` or project-level `.claude/settings.json`):

```json
{
  "mcpServers": {
    "graphthulhu": {
      "command": "graphthulhu",
      "env": {
        "LOGSEQ_API_URL": "http://127.0.0.1:12315",
        "LOGSEQ_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json` on macOS):

```json
{
  "mcpServers": {
    "graphthulhu": {
      "command": "graphthulhu",
      "env": {
        "LOGSEQ_API_URL": "http://127.0.0.1:12315",
        "LOGSEQ_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Cursor

Add to `.cursor/mcp.json` in your project:

```json
{
  "mcpServers": {
    "graphthulhu": {
      "command": "graphthulhu",
      "env": {
        "LOGSEQ_API_URL": "http://127.0.0.1:12315",
        "LOGSEQ_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Read-only mode

To disable all write operations (create, update, delete, move):

```json
{
  "mcpServers": {
    "graphthulhu": {
      "command": "graphthulhu",
      "args": ["--read-only"],
      "env": {
        "LOGSEQ_API_URL": "http://127.0.0.1:12315",
        "LOGSEQ_API_TOKEN": "your-token-here"
      }
    }
  }
}
```

### Version control warning

On startup, graphthulhu checks if your Logseq graph directory is git-controlled. If not, it prints a warning to stderr suggesting you initialize version control. Write operations cannot be undone without it.

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOGSEQ_API_URL` | `http://127.0.0.1:12315` | Logseq HTTP API endpoint |
| `LOGSEQ_API_TOKEN` | (required) | Bearer token from Logseq settings |

## Architecture

```
main.go              Entry point — creates client and server, runs stdio
server.go            MCP server setup — registers all 27 tools
client/logseq.go     Logseq HTTP API client with retry/backoff
tools/
  navigate.go        Page, block, links, references, BFS traversal
  search.go          Full-text, property, DataScript, tag search
  analyze.go         Graph overview, connections, gaps, clusters
  write.go           Create, update, delete, move, link operations
  journal.go         Date range and search within journals
  flashcard.go       SRS overview, due cards, card creation
  whiteboard.go      List and inspect whiteboards
  helpers.go         Result formatting utilities
graph/
  builder.go         In-memory graph construction from all pages
  algorithms.go      Overview, connections, gaps, clusters, BFS
parser/content.go    Regex extraction of [[links]], ((refs)), #tags, properties
types/
  logseq.go          Logseq API types with custom JSON unmarshaling
  tools.go           Input types for all 27 tools
```

### Design decisions

- **Full block trees, not flat text.** Every page read returns the complete nested hierarchy with parsed metadata on every block.
- **Context with every search result.** Search doesn't just return matching blocks — it includes the parent chain and siblings so the AI understands where the result sits.
- **In-memory graph for analysis.** Analysis tools build the full link graph in memory for BFS, connected components, and gap detection. This keeps per-query latency low.
- **DataScript as escape hatch.** When the built-in tools don't cover a query, `query_datalog` lets you run arbitrary Datalog against the Logseq database.
- **Content parsing on every block.** The parser extracts `[[links]]`, `((block refs))`, `#tags`, `key:: value` properties, task markers, and priorities from raw block content.

## Development

```bash
go build -o graphthulhu .          # Build
go test ./...                       # Test
go vet ./...                        # Vet
```

## License

[MIT](LICENSE)
