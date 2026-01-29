package types

// --- Navigate tool inputs ---

type GetPageInput struct {
	Name  string `json:"name" jsonschema:"Page name to retrieve"`
	Depth int    `json:"depth,omitempty" jsonschema:"Max block tree depth (-1 for unlimited). Default: -1"`
}

type GetBlockInput struct {
	UUID             string `json:"uuid" jsonschema:"Block UUID to retrieve"`
	IncludeAncestors bool   `json:"includeAncestors,omitempty" jsonschema:"Include ancestor chain from root. Default: true"`
	IncludeSiblings  bool   `json:"includeSiblings,omitempty" jsonschema:"Include sibling blocks. Default: false"`
}

type ListPagesInput struct {
	Namespace   string `json:"namespace,omitempty" jsonschema:"Filter by namespace prefix (e.g. graphthulhu)"`
	HasProperty string `json:"hasProperty,omitempty" jsonschema:"Filter to pages with this property key"`
	HasTag      string `json:"hasTag,omitempty" jsonschema:"Filter to pages with this tag"`
	SortBy      string `json:"sortBy,omitempty" jsonschema:"Sort by: name or modified or created. Default: name"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Max results. Default: 50"`
}

type GetLinksInput struct {
	Name      string `json:"name" jsonschema:"Page name to get links for"`
	Direction string `json:"direction,omitempty" jsonschema:"Link direction: forward or backward or both. Default: both"`
}

type GetReferencesInput struct {
	UUID string `json:"uuid" jsonschema:"Block UUID to find references for"`
}

type TraverseInput struct {
	From    string `json:"from" jsonschema:"Starting page name"`
	To      string `json:"to" jsonschema:"Target page name"`
	MaxHops int    `json:"maxHops,omitempty" jsonschema:"Maximum traversal depth. Default: 4"`
}

// --- Search tool inputs ---

type SearchInput struct {
	Query        string `json:"query" jsonschema:"Search text to find across all blocks"`
	ContextLines int    `json:"contextLines,omitempty" jsonschema:"Number of parent/sibling blocks for context. Default: 2"`
	Limit        int    `json:"limit,omitempty" jsonschema:"Max results. Default: 20"`
}

type QueryPropertiesInput struct {
	Property string `json:"property" jsonschema:"Property key to search for"`
	Value    string `json:"value,omitempty" jsonschema:"Property value to match (omit to find all with this property)"`
	Operator string `json:"operator,omitempty" jsonschema:"Comparison: eq or contains or gt or lt. Default: eq"`
}

type QueryDatalogInput struct {
	Query  string `json:"query" jsonschema:"Datalog/DataScript query string"`
	Inputs []any  `json:"inputs,omitempty" jsonschema:"Query input bindings"`
}

type FindByTagInput struct {
	Tag             string `json:"tag" jsonschema:"Tag name to search for"`
	IncludeChildren bool   `json:"includeChildren,omitempty" jsonschema:"Include child tags in hierarchy. Default: true"`
}

// --- Analyze tool inputs ---

// GraphOverviewInput has no required params — returns global stats.
type GraphOverviewInput struct{}

type FindConnectionsInput struct {
	From     string `json:"from" jsonschema:"Starting page name"`
	To       string `json:"to" jsonschema:"Target page name"`
	MaxDepth int    `json:"maxDepth,omitempty" jsonschema:"Max search depth. Default: 5"`
}

// KnowledgeGapsInput has no required params — returns sparse areas.
type KnowledgeGapsInput struct{}

// TopicClustersInput has no required params — returns community clusters.
type TopicClustersInput struct{}

// --- Write tool inputs ---

type CreatePageInput struct {
	Name       string         `json:"name" jsonschema:"Page name to create"`
	Properties map[string]any `json:"properties,omitempty" jsonschema:"Page properties as key-value pairs"`
	Blocks     []string       `json:"blocks,omitempty" jsonschema:"Initial block contents to add to the page"`
}

type UpsertBlocksInput struct {
	Page     string       `json:"page" jsonschema:"Page name to add blocks to"`
	Blocks   []BlockInput `json:"blocks" jsonschema:"Blocks to create or update"`
	Position string       `json:"position,omitempty" jsonschema:"Where to add: append or prepend. Default: append"`
}

type BlockInput struct {
	Content    string            `json:"content" jsonschema:"Block text content"`
	Properties map[string]string `json:"properties,omitempty" jsonschema:"Block properties"`
	Children   []BlockInput      `json:"children,omitempty" jsonschema:"Nested child blocks"`
}

type UpdateBlockInput struct {
	UUID    string `json:"uuid" jsonschema:"UUID of block to update"`
	Content string `json:"content" jsonschema:"New content for the block (replaces existing content entirely)"`
}

type DeleteBlockInput struct {
	UUID string `json:"uuid" jsonschema:"UUID of block to delete"`
}

type MoveBlockInput struct {
	UUID       string `json:"uuid" jsonschema:"UUID of block to move"`
	TargetUUID string `json:"targetUuid" jsonschema:"UUID of target block"`
	Position   string `json:"position,omitempty" jsonschema:"Placement: before or after or child. Default: child"`
}

type LinkPagesInput struct {
	From    string `json:"from" jsonschema:"Source page name"`
	To      string `json:"to" jsonschema:"Target page name"`
	Context string `json:"context,omitempty" jsonschema:"Description of the relationship between pages"`
}

// --- Flashcard tool inputs ---

// FlashcardOverviewInput has no required params — returns SRS statistics.
type FlashcardOverviewInput struct{}

type FlashcardDueInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max cards to return. Default: 20"`
}

type FlashcardCreateInput struct {
	Page  string `json:"page" jsonschema:"Page to create the card on"`
	Front string `json:"front" jsonschema:"Front of the card (question)"`
	Back  string `json:"back" jsonschema:"Back of the card (answer)"`
}

// --- Whiteboard tool inputs ---

// ListWhiteboardsInput has no required params.
type ListWhiteboardsInput struct{}

type GetWhiteboardInput struct {
	Name string `json:"name" jsonschema:"Whiteboard name"`
}

// --- Journal tool inputs ---

type JournalRangeInput struct {
	From          string `json:"from" jsonschema:"Start date (YYYY-MM-DD)"`
	To            string `json:"to" jsonschema:"End date (YYYY-MM-DD)"`
	IncludeBlocks bool   `json:"includeBlocks,omitempty" jsonschema:"Include full block trees. Default: true"`
}

type JournalSearchInput struct {
	Query string `json:"query" jsonschema:"Text to search for in journal entries"`
	From  string `json:"from,omitempty" jsonschema:"Start date filter (YYYY-MM-DD)"`
	To    string `json:"to,omitempty" jsonschema:"End date filter (YYYY-MM-DD)"`
}
