# graphthulhu

MCP server that gives AI full access to your knowledge graph. Supports **Logseq** and **Obsidian** — both with full read-write support. Navigate pages, search blocks, analyze link structure, track decisions, manage flashcards, and write content — all through the [Model Context Protocol](https://modelcontextprotocol.io).

Built in Go with the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk).

## Why

Your knowledge graph stores interconnected pages, blocks, and links. But AI assistants can't see any of it — they're blind to your second brain.

graphthulhu fixes that. It exposes your entire knowledge graph through MCP, so Claude (or any MCP client) can:

- Read any page with its full block tree, parsed links, tags, and properties
- Search across all blocks with contextual results (parent chain + siblings)
- Traverse the link graph to discover how concepts connect
- Find knowledge gaps — orphan pages, dead ends, weakly-linked areas
- Discover topic clusters through connected component analysis
- Create pages, write blocks, build hierarchies, link pages bidirectionally (Logseq)
- Query with raw DataScript/Datalog for anything the built-in tools don't cover (Logseq)
- Review flashcards with spaced repetition statistics (Logseq)
- Explore whiteboards and their spatial connections (Logseq)

It turns "tell me about X" into an AI that actually understands your knowledge graph's structure.

## Tools

37 tools across 9 categories. Most work with both backends; some are Logseq-only (DataScript queries, flashcards, whiteboards).

### Navigate

| Tool | Backend | Description |
|------|---------|-------------|
| `get_page` | Both | Full recursive block tree with parsed links, tags, properties |
| `get_block` | Both | Block by UUID with ancestor chain, children, siblings |
| `list_pages` | Both | Filter by namespace, property, or tag; sort by name/modified/created |
| `get_links` | Both | Forward and backward links with the blocks that contain them |
| `get_references` | Logseq | All blocks referencing a specific block via `((uuid))` |
| `traverse` | Both | BFS path-finding between two pages through the link graph |

### Search

| Tool | Backend | Description |
|------|---------|-------------|
| `search` | Both | Full-text search with parent chain + sibling context |
| `query_properties` | Both | Find by property values with operators (eq, contains, gt, lt) |
| `query_datalog` | Logseq | Raw DataScript/Datalog queries against the Logseq database |
| `find_by_tag` | Both | Tag search with child tag hierarchy support |

### Analyze

| Tool | Backend | Description |
|------|---------|-------------|
| `graph_overview` | Both | Global stats: pages, blocks, links, most connected, namespaces |
| `find_connections` | Both | Direct links, shortest paths, shared connections between pages |
| `knowledge_gaps` | Both | Orphan pages, dead ends, weakly-linked areas |
| `list_orphans` | Both | List orphan page names with block counts and property status |
| `topic_clusters` | Both | Connected components with hub identification |

### Write

| Tool | Backend | Description |
|------|---------|-------------|
| `create_page` | Both | New page with properties and initial blocks |
| `append_blocks` | Both | Append plain-text blocks (simpler than upsert_blocks) |
| `upsert_blocks` | Both | Batch create with nested children for deep hierarchies |
| `update_block` | Both | Replace block content by UUID |
| `delete_block` | Both | Remove block and all children |
| `move_block` | Both | Reposition before, after, or as child of another block (cross-page supported) |
| `link_pages` | Both | Bidirectional link with optional relationship context |
| `delete_page` | Both | Remove a page and all its blocks |
| `rename_page` | Both | Rename page and update all `[[links]]` across the graph |
| `bulk_update_properties` | Both | Set a property on multiple pages in one call |

### Decision

| Tool | Backend | Description |
|------|---------|-------------|
| `decision_check` | Both | Surface open, overdue, and resolved decisions with deadline status |
| `decision_create` | Both | Create a DECIDE block with `#decision` tag, deadline, options, context |
| `decision_resolve` | Both | Mark a decision as DONE with resolution date and outcome |
| `decision_defer` | Both | Push deadline with reason, tracks deferral count, warns after 3+ |
| `analysis_health` | Both | Audit analysis/strategy pages for graph connectivity (3+ links or has decision) |

### Journal

| Tool | Backend | Description |
|------|---------|-------------|
| `journal_range` | Both | Entries across a date range with full block trees |
| `journal_search` | Both | Search within journals, optionally filtered by date |

### Flashcard

| Tool | Backend | Description |
|------|---------|-------------|
| `flashcard_overview` | Logseq | SRS stats: total, due, new vs reviewed, average repeats |
| `flashcard_due` | Logseq | Cards due for review with ease factor and interval |
| `flashcard_create` | Logseq | Create front/back card with `#card` tag |

### Whiteboard

| Tool | Backend | Description |
|------|---------|-------------|
| `list_whiteboards` | Logseq | All whiteboards in the graph |
| `get_whiteboard` | Logseq | Embedded pages, block references, visual connections |

### Health

| Tool | Backend | Description |
|------|---------|-------------|
| `health` | Both | Check server status: version, backend, read-only mode, page count |

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

### Setup: Logseq

1. In Logseq, go to **Settings → Features** and enable **HTTP APIs server**
2. Click the **API** icon that appears in the top toolbar
3. Click **Start Server**
4. Click **Create Token** and copy the generated token — you'll need it for configuration

The API runs on `http://127.0.0.1:12315` by default.

### Setup: Obsidian

No plugins or server required. graphthulhu reads your vault's `.md` files directly.

You need to provide the path to your vault:

```bash
graphthulhu serve --backend obsidian --vault /path/to/your/vault
```

Or via environment variables:

```bash
export GRAPHTHULHU_BACKEND=obsidian
export OBSIDIAN_VAULT_PATH=/path/to/your/vault
graphthulhu
```

The Obsidian backend supports full read-write operations. It parses YAML frontmatter into properties, builds a block tree from headings, and indexes `[[wikilinks]]` for backlink resolution. Writes use atomic temp-file renames, and the in-memory index is rebuilt after every mutation. File watching (fsnotify) keeps the index in sync with external edits. Daily notes are detected from a configurable subfolder (default: `daily notes`).

## Configuration

### Logseq — Claude Code

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

### Logseq — Claude Desktop

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

### Obsidian — Claude Code

```json
{
  "mcpServers": {
    "graphthulhu": {
      "command": "graphthulhu",
      "args": ["--backend", "obsidian", "--vault", "/path/to/your/vault"],
      "env": {}
    }
  }
}
```

### Obsidian — Claude Desktop

```json
{
  "mcpServers": {
    "graphthulhu": {
      "command": "graphthulhu",
      "args": ["--backend", "obsidian", "--vault", "/path/to/your/vault"],
      "env": {}
    }
  }
}
```

### Read-only mode

To disable all write operations (Logseq — Obsidian is always read-only):

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

On startup with the Logseq backend, graphthulhu checks if your graph directory is git-controlled. If not, it prints a warning to stderr suggesting you initialize version control. Write operations cannot be undone without it.

### Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOGSEQ_API_URL` | `http://127.0.0.1:12315` | Logseq HTTP API endpoint |
| `LOGSEQ_API_TOKEN` | (required for Logseq) | Bearer token from Logseq settings |
| `GRAPHTHULHU_BACKEND` | `logseq` | Backend type: `logseq` or `obsidian` |
| `OBSIDIAN_VAULT_PATH` | — | Path to Obsidian vault root |

## Architecture

```
main.go              Entry point — backend routing, MCP server startup
cli.go               CLI subcommands: journal, add, search
server.go            MCP server setup — conditional tool registration
backend/backend.go   Backend interface + optional capability interfaces
client/logseq.go     Logseq HTTP API client with retry/backoff
vault/
  vault.go           Obsidian vault client — reads .md files into Backend interface
  markdown.go        Markdown → block tree parser (heading-based sectioning)
  frontmatter.go     YAML frontmatter parser
  index.go           Backlink index builder from [[wikilinks]]
tools/
  navigate.go        Page, block, links, references, BFS traversal
  search.go          Full-text, property, DataScript/frontmatter, tag search
  analyze.go         Graph overview, connections, gaps, clusters
  write.go           Create, update, delete, move, link operations
  decision.go        Decision protocol: check, create, resolve, defer, analysis health
  journal.go         Date range and search within journals
  flashcard.go       SRS overview, due cards, card creation
  whiteboard.go      List and inspect whiteboards
  helpers.go         Result formatting utilities
graph/
  builder.go         In-memory graph construction from any backend
  algorithms.go      Overview, connections, gaps, clusters, BFS
parser/content.go    Regex extraction of [[links]], ((refs)), #tags, properties
types/
  logseq.go          Shared types with custom JSON unmarshaling
  tools.go           Input types for all 32 tools
```

### Design decisions

- **Backend interface.** All tools program against `backend.Backend`, not a concrete client. Adding a new backend means implementing the interface — no tool changes needed.
- **Full block trees, not flat text.** Every page read returns the complete nested hierarchy with parsed metadata on every block.
- **Context with every search result.** Search doesn't just return matching blocks — it includes the parent chain and siblings so the AI understands where the result sits.
- **In-memory graph for analysis.** Analysis tools build the full link graph in memory for BFS, connected components, and gap detection. This keeps per-query latency low.
- **Optional capability interfaces.** Tools like `query_properties` and `find_by_tag` check if the backend implements `PropertySearcher` or `TagSearcher` at runtime, falling back to DataScript for Logseq. This lets Obsidian use file scanning while Logseq keeps its Datalog queries.
- **DataScript as escape hatch.** When the built-in tools don't cover a query, `query_datalog` lets you run arbitrary Datalog against the Logseq database.
- **Content parsing on every block.** The parser extracts `[[links]]`, `((block refs))`, `#tags`, `key:: value` properties, task markers, and priorities from raw block content.
- **Heading-based blocks for Obsidian.** Obsidian markdown is sectioned by headings (H1-H6) into a hierarchical block tree. Block UUIDs are persisted via `<!-- id: UUID -->` HTML comments for stability across edits, with deterministic fallback for files without embedded IDs.
- **File watching.** The Obsidian backend watches the vault directory with fsnotify and selectively re-indexes changed files, keeping the in-memory index in sync with external edits.

## Development

```bash
go build -o graphthulhu .          # Build
go test ./...                       # Test
go vet ./...                        # Vet
```

## License

[MIT](LICENSE)
