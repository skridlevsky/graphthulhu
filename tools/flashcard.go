package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/skridlevsky/graphthulhu/client"
	"github.com/skridlevsky/graphthulhu/types"
)

// Flashcard implements flashcard/SRS MCP tools.
type Flashcard struct {
	client *client.Client
}

// NewFlashcard creates a new Flashcard tool handler.
func NewFlashcard(c *client.Client) *Flashcard {
	return &Flashcard{client: c}
}

type cardData struct {
	UUID       string         `json:"uuid"`
	Content    string         `json:"content"`
	Page       string         `json:"page,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
	IsDue      bool           `json:"isDue"`
}

// FlashcardOverview returns SRS statistics across all cards.
func (f *Flashcard) FlashcardOverview(ctx context.Context, req *mcp.CallToolRequest, input types.FlashcardOverviewInput) (*mcp.CallToolResult, any, error) {
	cards, err := f.getAllCards(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to query flashcards: %v", err)), nil, nil
	}

	if len(cards) == 0 {
		return textResult("No flashcards found in the graph. Create cards by adding #card to blocks in Logseq."), nil, nil
	}

	now := time.Now()
	var dueCount, newCount, reviewedCount int
	var totalRepeats float64

	for _, c := range cards {
		props := c.Properties
		if props == nil {
			newCount++
			continue
		}

		if _, ok := props["card-next-schedule"]; !ok {
			newCount++
			continue
		}

		reviewedCount++
		if repeats, ok := props["card-repeats"]; ok {
			if r, ok := repeats.(float64); ok {
				totalRepeats += r
			}
		}

		if f.isCardDue(props, now) {
			dueCount++
		}
	}

	avgRepeats := 0.0
	if reviewedCount > 0 {
		avgRepeats = totalRepeats / float64(reviewedCount)
	}

	res, err := jsonTextResult(map[string]any{
		"totalCards":     len(cards),
		"dueNow":        dueCount,
		"newCards":       newCount,
		"reviewedCards":  reviewedCount,
		"averageRepeats": fmt.Sprintf("%.1f", avgRepeats),
	})
	return res, nil, err
}

// FlashcardDue returns cards currently due for review.
func (f *Flashcard) FlashcardDue(ctx context.Context, req *mcp.CallToolRequest, input types.FlashcardDueInput) (*mcp.CallToolResult, any, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	cards, err := f.getAllCards(ctx)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to query flashcards: %v", err)), nil, nil
	}

	now := time.Now()
	var due []cardData

	for _, c := range cards {
		if len(due) >= limit {
			break
		}
		// New cards (never reviewed) are always due
		if c.Properties == nil || c.Properties["card-next-schedule"] == nil {
			c.IsDue = true
			due = append(due, c)
			continue
		}
		if f.isCardDue(c.Properties, now) {
			c.IsDue = true
			due = append(due, c)
		}
	}

	if len(due) == 0 {
		return textResult("No cards due for review right now."), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"dueCount": len(due),
		"cards":    due,
	})
	return res, nil, err
}

// FlashcardCreate creates a new flashcard (block with #card tag and child answer).
func (f *Flashcard) FlashcardCreate(ctx context.Context, req *mcp.CallToolRequest, input types.FlashcardCreateInput) (*mcp.CallToolResult, any, error) {
	frontContent := input.Front + " #card"

	frontBlock, err := f.client.AppendBlockInPage(ctx, input.Page, frontContent)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create card front: %v", err)), nil, nil
	}

	if frontBlock == nil {
		return errorResult("created card but got no block reference"), nil, nil
	}

	_, err = f.client.InsertBlock(ctx, frontBlock.UUID, input.Back, map[string]any{
		"isPageBlock": false,
	})
	if err != nil {
		return errorResult(fmt.Sprintf("created front but failed to add answer: %v", err)), nil, nil
	}

	res, err := jsonTextResult(map[string]any{
		"created": true,
		"page":    input.Page,
		"uuid":    frontBlock.UUID,
		"front":   input.Front,
		"back":    input.Back,
	})
	return res, nil, err
}

// --- Internal helpers ---

func (f *Flashcard) getAllCards(ctx context.Context) ([]cardData, error) {
	query := `[:find (pull ?b [:block/uuid :block/content :block/properties
	                           {:block/page [:block/name :block/original-name]}])
		:where
		[?b :block/refs ?ref]
		[?ref :block/name "card"]]`

	raw, err := f.client.DatascriptQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	var results [][]json.RawMessage
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}

	var cards []cardData
	for _, r := range results {
		if len(r) == 0 {
			continue
		}
		var block struct {
			UUID       string         `json:"uuid"`
			Content    string         `json:"content"`
			Properties map[string]any `json:"properties"`
			Page       *struct {
				Name         string `json:"name"`
				OriginalName string `json:"original-name"`
			} `json:"page"`
		}
		if err := json.Unmarshal(r[0], &block); err != nil {
			continue
		}

		cd := cardData{
			UUID:       block.UUID,
			Content:    block.Content,
			Properties: block.Properties,
		}
		if block.Page != nil {
			cd.Page = block.Page.OriginalName
			if cd.Page == "" {
				cd.Page = block.Page.Name
			}
		}
		cards = append(cards, cd)
	}

	return cards, nil
}

func (f *Flashcard) isCardDue(props map[string]any, now time.Time) bool {
	schedStr, ok := props["card-next-schedule"].(string)
	if !ok {
		return true // no schedule = due
	}

	// Try multiple date formats Logseq might use
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, fmt := range formats {
		if t, err := time.Parse(fmt, schedStr); err == nil {
			return now.After(t)
		}
	}

	return true // unparseable = treat as due
}
