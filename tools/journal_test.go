package tools

import (
	"reflect"
	"testing"
	"time"
)

func TestOrdinalSuffix(t *testing.T) {
	tests := []struct {
		day  int
		want string
	}{
		{1, "st"},
		{2, "nd"},
		{3, "rd"},
		{4, "th"},
		{11, "th"},
		{12, "th"},
		{13, "th"},
		{21, "st"},
		{22, "nd"},
		{23, "rd"},
		{24, "th"},
		{31, "st"},
	}

	for _, tt := range tests {
		t.Run(time.Date(2026, time.January, tt.day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), func(t *testing.T) {
			got := ordinalSuffix(tt.day)
			if got != tt.want {
				t.Fatalf("ordinalSuffix(%d) = %q, want %q", tt.day, got, tt.want)
			}
		})
	}
}

func TestJournalPageNames_UsesCorrectOrdinalSuffix(t *testing.T) {
	d := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC)
	got := journalPageNames(d)
	want := []string{
		"Feb 15th, 2026",
		"February 15th, 2026",
		"2026-02-15",
		"February 15, 2026",
		"15th Feb 2026",
		"15th February 2026",
		"15 Feb 2026",
		"15 February 2026",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("journalPageNames(%s) = %#v, want %#v", d.Format("2006-01-02"), got, want)
	}
}

// TestJournalPageNames_DayFirstFormats verifies the day-first variants for
// users with non-default Logseq date formats (issue #9).
func TestJournalPageNames_DayFirstFormats(t *testing.T) {
	tests := []struct {
		date time.Time
		want []string // formats that must appear in the candidate list
	}{
		{
			// Issue #9 reporter's format: "16th Apr 2026"
			date: time.Date(2026, time.April, 16, 0, 0, 0, 0, time.UTC),
			want: []string{"16th Apr 2026", "16th April 2026", "16 Apr 2026", "16 April 2026"},
		},
		{
			// Day with "nd" suffix
			date: time.Date(2026, time.July, 2, 0, 0, 0, 0, time.UTC),
			want: []string{"2nd Jul 2026", "2nd July 2026", "2 Jul 2026", "2 July 2026"},
		},
		{
			// 11th-13th edge case (all use "th")
			date: time.Date(2026, time.December, 11, 0, 0, 0, 0, time.UTC),
			want: []string{"11th Dec 2026", "11th December 2026", "11 Dec 2026", "11 December 2026"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.date.Format("2006-01-02"), func(t *testing.T) {
			got := journalPageNames(tt.date)
			gotSet := make(map[string]bool, len(got))
			for _, name := range got {
				gotSet[name] = true
			}
			for _, w := range tt.want {
				if !gotSet[w] {
					t.Errorf("expected %q in journalPageNames(%s), got %#v", w, tt.date.Format("2006-01-02"), got)
				}
			}
		})
	}
}
