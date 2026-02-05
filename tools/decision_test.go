package tools

import (
	"testing"
	"time"

	"github.com/skridlevsky/graphthulhu/types"
)

func intPtr(i int) *int { return &i }

func TestUpsertProperty(t *testing.T) {
	tests := []struct {
		name    string
		content string
		key     string
		value   string
		want    string
	}{
		{
			name:    "add new property",
			content: "DECIDE something #decision",
			key:     "deadline",
			value:   "2026-03-01",
			want:    "DECIDE something #decision\ndeadline:: 2026-03-01",
		},
		{
			name:    "update existing property",
			content: "DECIDE something #decision\ndeadline:: 2026-01-15",
			key:     "deadline",
			value:   "2026-03-01",
			want:    "DECIDE something #decision\ndeadline:: 2026-03-01",
		},
		{
			name:    "update one among several",
			content: "DECIDE something #decision\ndeadline:: 2026-01-15\noptions:: a, b, c\ncontext:: some context",
			key:     "deadline",
			value:   "2026-03-01",
			want:    "DECIDE something #decision\ndeadline:: 2026-03-01\noptions:: a, b, c\ncontext:: some context",
		},
		{
			name:    "add second property",
			content: "DECIDE something #decision\ndeadline:: 2026-03-01",
			key:     "resolved",
			value:   "2026-01-29",
			want:    "DECIDE something #decision\ndeadline:: 2026-03-01\nresolved:: 2026-01-29",
		},
		{
			name:    "value with special characters",
			content: "DECIDE something #decision",
			key:     "outcome",
			value:   "Show HN first \u2014 highest signal-to-noise",
			want:    "DECIDE something #decision\noutcome:: Show HN first \u2014 highest signal-to-noise",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := upsertProperty(tt.content, tt.key, tt.value)
			if got != tt.want {
				t.Errorf("upsertProperty() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestCountLinksInTree(t *testing.T) {
	tests := []struct {
		name   string
		blocks []types.BlockEntity
		want   int
	}{
		{
			name:   "empty",
			blocks: nil,
			want:   0,
		},
		{
			name: "flat blocks with links",
			blocks: []types.BlockEntity{
				{Content: "See [[PageA]] and [[PageB]]"},
				{Content: "Also [[PageC]]"},
			},
			want: 3,
		},
		{
			name: "no links",
			blocks: []types.BlockEntity{
				{Content: "plain text"},
				{Content: "more plain text"},
			},
			want: 0,
		},
		{
			name: "nested blocks with links",
			blocks: []types.BlockEntity{
				{
					Content: "Parent [[Link1]]",
					Children: []types.BlockEntity{
						{Content: "Child [[Link2]] and [[Link3]]"},
						{
							Content: "Another child",
							Children: []types.BlockEntity{
								{Content: "Grandchild [[Link4]]"},
							},
						},
					},
				},
			},
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLinksInTree(tt.blocks)
			if got != tt.want {
				t.Errorf("countLinksInTree() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseDecisionBlock(t *testing.T) {
	today := time.Date(2026, 1, 29, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		block        types.BlockEntity
		wantOK       bool
		wantMarker   string
		wantPage     string
		wantDeadline string
		wantResolved string
		wantOutcome  string
		wantOverdue  bool
		wantDeferred int
		wantDaysLeft *int
	}{
		{
			name: "DECIDE with future deadline",
			block: types.BlockEntity{
				UUID:    "uuid-1",
				Content: "DECIDE Should we launch? #decision\ndeadline:: 2026-02-15",
				Page:    &types.PageRef{ID: 1, Name: "test-page"},
			},
			wantOK:       true,
			wantMarker:   "DECIDE",
			wantPage:     "test-page",
			wantDeadline: "2026-02-15",
			wantOverdue:  false,
			wantDaysLeft: intPtr(17),
		},
		{
			name: "DONE with resolved date",
			block: types.BlockEntity{
				UUID:    "uuid-2",
				Content: "DONE Passive growth #decision\ndeadline:: 2026-01-14\nresolved:: 2026-01-29",
				Page:    &types.PageRef{ID: 2, Name: "floustate"},
			},
			wantOK:       true,
			wantMarker:   "DONE",
			wantPage:     "floustate",
			wantDeadline: "2026-01-14",
			wantResolved: "2026-01-29",
			wantOverdue:  false,
			wantDaysLeft: nil, // resolved decisions don't compute daysLeft
		},
		{
			name: "no marker rejects documentation mention",
			block: types.BlockEntity{
				UUID:    "uuid-3",
				Content: "We use #decision tags for tracking decisions",
				Page:    &types.PageRef{ID: 3, Name: "docs"},
			},
			wantOK: false,
		},
		{
			name: "overdue deadline",
			block: types.BlockEntity{
				UUID:    "uuid-4",
				Content: "DECIDE Fix the bug #decision\ndeadline:: 2026-01-15",
				Page:    &types.PageRef{ID: 4, Name: "bugs"},
			},
			wantOK:       true,
			wantMarker:   "DECIDE",
			wantPage:     "bugs",
			wantDeadline: "2026-01-15",
			wantOverdue:  true,
			wantDaysLeft: intPtr(-14),
		},
		{
			name: "deferred decision preserves count",
			block: types.BlockEntity{
				UUID:    "uuid-5",
				Content: "DECIDE Launch strategy #decision\ndeadline:: 2026-03-01\ndeferred:: 2\ndeferred-on:: 2026-01-20",
				Page:    &types.PageRef{ID: 5, Name: "strategy"},
			},
			wantOK:       true,
			wantMarker:   "DECIDE",
			wantPage:     "strategy",
			wantDeadline: "2026-03-01",
			wantOverdue:  false,
			wantDeferred: 2,
			wantDaysLeft: intPtr(31),
		},
		{
			name: "resolved with outcome",
			block: types.BlockEntity{
				UUID:    "uuid-6",
				Content: "DONE Choose framework #decision\ndeadline:: 2026-01-20\nresolved:: 2026-01-25\noutcome:: Go with React",
				Page:    &types.PageRef{ID: 6, Name: "tech"},
			},
			wantOK:       true,
			wantMarker:   "DONE",
			wantPage:     "tech",
			wantDeadline: "2026-01-20",
			wantResolved: "2026-01-25",
			wantOutcome:  "Go with React",
			wantOverdue:  false,
			wantDaysLeft: nil,
		},
		{
			name: "bare marker without trailing text",
			block: types.BlockEntity{
				UUID:    "uuid-7",
				Content: "DECIDE",
			},
			wantOK:     true,
			wantMarker: "DECIDE",
		},
		{
			name: "TODO marker is valid",
			block: types.BlockEntity{
				UUID:    "uuid-8",
				Content: "TODO review #decision\ndeadline:: 2026-02-01",
			},
			wantOK:       true,
			wantMarker:   "TODO",
			wantDeadline: "2026-02-01",
			wantDaysLeft: intPtr(3),
		},
		{
			name: "WAIT marker is valid",
			block: types.BlockEntity{
				UUID:    "uuid-9",
				Content: "WAIT pending info #decision\ndeadline:: 2026-02-10",
			},
			wantOK:       true,
			wantMarker:   "WAIT",
			wantDeadline: "2026-02-10",
			wantDaysLeft: intPtr(12),
		},
		{
			name: "nil page leaves page empty",
			block: types.BlockEntity{
				UUID:    "uuid-10",
				Content: "DECIDE something #decision\ndeadline:: 2026-02-01",
			},
			wantOK:       true,
			wantMarker:   "DECIDE",
			wantPage:     "",
			wantDeadline: "2026-02-01",
			wantDaysLeft: intPtr(3),
		},
		{
			name: "deadline today is not overdue",
			block: types.BlockEntity{
				UUID:    "uuid-11",
				Content: "DECIDE urgent #decision\ndeadline:: 2026-01-29",
			},
			wantOK:       true,
			wantMarker:   "DECIDE",
			wantDeadline: "2026-01-29",
			wantOverdue:  false,
			wantDaysLeft: intPtr(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDecisionBlock(tt.block, today)
			if ok != tt.wantOK {
				t.Fatalf("parseDecisionBlock() ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if got.Marker != tt.wantMarker {
				t.Errorf("Marker = %q, want %q", got.Marker, tt.wantMarker)
			}
			if got.Page != tt.wantPage {
				t.Errorf("Page = %q, want %q", got.Page, tt.wantPage)
			}
			if got.Deadline != tt.wantDeadline {
				t.Errorf("Deadline = %q, want %q", got.Deadline, tt.wantDeadline)
			}
			if got.Resolved != tt.wantResolved {
				t.Errorf("Resolved = %q, want %q", got.Resolved, tt.wantResolved)
			}
			if got.Outcome != tt.wantOutcome {
				t.Errorf("Outcome = %q, want %q", got.Outcome, tt.wantOutcome)
			}
			if got.Overdue != tt.wantOverdue {
				t.Errorf("Overdue = %v, want %v", got.Overdue, tt.wantOverdue)
			}
			if got.Deferred != tt.wantDeferred {
				t.Errorf("Deferred = %d, want %d", got.Deferred, tt.wantDeferred)
			}
			if tt.wantDaysLeft == nil {
				if got.DaysLeft != nil {
					t.Errorf("DaysLeft = %d, want nil", *got.DaysLeft)
				}
			} else {
				if got.DaysLeft == nil {
					t.Errorf("DaysLeft = nil, want %d", *tt.wantDaysLeft)
				} else if *got.DaysLeft != *tt.wantDaysLeft {
					t.Errorf("DaysLeft = %d, want %d", *got.DaysLeft, *tt.wantDaysLeft)
				}
			}
		})
	}
}
