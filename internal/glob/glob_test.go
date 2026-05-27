package glob

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		// literal
		{"exact match", "foo.csv", "foo.csv", true},
		{"literal mismatch", "foo.csv", "bar.csv", false},

		// single * within a segment
		{"star extension", "*.csv", "orders.csv", true},
		{"star extension no match", "*.csv", "orders.json", false},
		{"star prefix", "orders_*", "orders_shopify", true},
		{"star middle", "a*c", "abc", true},
		{"star matches empty", "a*c", "ac", true},
		{"star does not cross segment", "*.csv", "data/orders.csv", false},
		{"bare star one segment", "*", "anything", true},
		{"bare star does not cross segment", "*", "a/b", false},

		// ? single char
		{"question single char", "?.csv", "a.csv", true},
		{"question requires a char", "?.csv", ".csv", false},
		{"question one char only", "a?c", "abc", true},
		{"question no match two chars", "a?c", "abbc", false},

		// ** across segments
		{"double star trailing matches all", "a/**", "a/b/c", true},
		{"double star trailing matches single", "a/**", "a/b", true},
		{"double star middle", "a/**/b", "a/x/y/b", true},
		{"double star middle zero dirs", "a/**/b", "a/b", true},
		{"double star middle no match", "a/**/b", "a/x/y/c", false},
		{"double star leading", "**/b", "x/y/b", true},
		{"double star leading single", "**/b", "b", true},
		{"consecutive double star collapse", "a/**/**/b", "a/x/y/b", true},
		{"only double star", "**", "a/b/c/d", true},

		// segment boundaries
		{"segment count mismatch shorter", "a/b", "a", false},
		{"segment count mismatch longer", "a/b", "a/b/c", false},
		{"nested literal", "a/b/c", "a/b/c", true},

		// combined wildcards
		{"star and question", "a*?", "abcd", true},
		{"double star with star file", "a/**/*.csv", "a/x/y/data.csv", true},
		{"double star with star file no match", "a/**/*.json", "a/x/data.csv", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.pattern, tt.input); got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}
