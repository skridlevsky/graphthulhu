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
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("journalPageNames(%s) = %#v, want %#v", d.Format("2006-01-02"), got, want)
	}
}
