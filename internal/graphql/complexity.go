package graphql

import (
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
)

// maxQueryComplexity is the maximum allowed cost score for incoming queries
// (SEC-11). Depth limiting alone does not stop wide queries (many sibling
// list fields with large page sizes), so a cost ceiling is enforced too.
const maxQueryComplexity = 2000

// Multiplier bounds for list fields. When a paging argument (pageSize/limit/
// first) is present its literal value is used, clamped to maxListMultiplier;
// when the argument's value is a variable (not statically known) the default
// is assumed.
const (
	defaultListMultiplier = 20
	maxListMultiplier     = 100
)

// queryComplexity parses a GraphQL query string and returns its cost score.
// Returns 0 on parse failure (let graphql.Do report the error).
//
// Cost model (heuristic, schema-agnostic):
//   - every field costs 1;
//   - a field with children adds multiplier × children-cost, where the
//     multiplier is the field's paging argument (pageSize/limit/first) when
//     statically known, otherwise 1 for plain nesting.
func queryComplexity(query string) int {
	src := source.NewSource(&source.Source{Body: []byte(query)})
	doc, err := parser.Parse(parser.ParseParams{Source: src})
	if err != nil {
		return 0
	}
	total := 0
	for _, def := range doc.Definitions {
		if op, ok := def.(*ast.OperationDefinition); ok {
			total += selectionCost(op.SelectionSet, doc)
		}
	}
	return total
}

// selectionCost recursively scores a SelectionSet, resolving inline fragments
// and named fragment spreads.
func selectionCost(set *ast.SelectionSet, doc *ast.Document) int {
	if set == nil || len(set.Selections) == 0 {
		return 0
	}
	cost := 0
	for _, sel := range set.Selections {
		switch v := sel.(type) {
		case *ast.Field:
			cost += 1 + fieldMultiplier(v)*selectionCost(v.SelectionSet, doc)
		case *ast.InlineFragment:
			cost += selectionCost(v.SelectionSet, doc)
		case *ast.FragmentSpread:
			if frag := findFragment(doc, v.Name.Value); frag != nil {
				cost += selectionCost(frag.SelectionSet, doc)
			}
		}
	}
	return cost
}

// fieldMultiplier derives the child-cost multiplier from a field's paging
// arguments. Fields without a paging argument multiply by 1 (plain nesting).
func fieldMultiplier(f *ast.Field) int {
	for _, arg := range f.Arguments {
		switch arg.Name.Value {
		case "pageSize", "page_size", "limit", "first":
			if iv, ok := arg.Value.(*ast.IntValue); ok {
				n := parseIntValue(iv.Value)
				if n < 1 {
					return 1
				}
				if n > maxListMultiplier {
					return maxListMultiplier
				}
				return n
			}
			// Variable or non-literal value: assume the default page size.
			return defaultListMultiplier
		}
	}
	return 1
}

// parseIntValue converts the AST's decimal string to int without strconv
// error handling noise; malformed values (impossible after parsing) yield 0.
func parseIntValue(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
		if n > 1<<30 {
			return 1 << 30
		}
	}
	return n
}
