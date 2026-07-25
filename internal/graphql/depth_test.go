package graphql

import "testing"

func TestQueryDepth(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"scalar field", `{ articles { total } }`, 2},
		{"nested 3", `{ articles { items { title } } }`, 3},
		{"deep 5", `{ articles { items { author { role { name } } } } }`, 5},
		{"inline fragment", `{ articles { items { ... on Article { tags { name } } } } }`, 4},
		{"fragment spread", `
			fragment F on Article { tags { name } }
			{ articles { items { ...F } } }
		`, 4},
		{"multiple top fields", `{ articles { total } tags { name } }`, 2},
		{"depth 1", `{ feed }`, 1},
		{"invalid query", `not valid graphql!!!`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := queryDepth(tc.query)
			if got != tc.want {
				t.Errorf("queryDepth() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestQueryDepth_ExceedsMax(t *testing.T) {
	// Build a query with depth 11 (exceeds maxQueryDepth=10).
	q := "{ a"
	for i := 0; i < 10; i++ {
		q += " { b"
	}
	for i := 0; i < 10; i++ {
		q += " }"
	}
	q += " }"
	depth := queryDepth(q)
	if depth <= maxQueryDepth {
		t.Errorf("expected depth > %d, got %d", maxQueryDepth, depth)
	}
}
