package graphql

import (
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
)

// maxQueryDepth is the maximum allowed nesting depth for incoming queries.
// Queries exceeding this are rejected before execution to prevent resource
// exhaustion from deeply nested selections.
const maxQueryDepth = 10

// queryDepth parses a GraphQL query string and returns the maximum selection
// nesting depth. Returns 0 on parse failure (let graphql.Do report the error).
func queryDepth(query string) int {
	src := source.NewSource(&source.Source{Body: []byte(query)})
	doc, err := parser.Parse(parser.ParseParams{Source: src})
	if err != nil {
		return 0
	}
	max := 0
	for _, def := range doc.Definitions {
		if op, ok := def.(*ast.OperationDefinition); ok {
			if d := selectionDepth(op.SelectionSet, doc); d > max {
				max = d
			}
		}
	}
	return max
}

// selectionDepth recursively computes the maximum depth of a SelectionSet,
// resolving inline fragments and named fragment spreads.
func selectionDepth(set *ast.SelectionSet, doc *ast.Document) int {
	if set == nil || len(set.Selections) == 0 {
		return 0
	}
	max := 0
	for _, sel := range set.Selections {
		var d int
		switch v := sel.(type) {
		case *ast.Field:
			d = 1 + selectionDepth(v.SelectionSet, doc)
		case *ast.InlineFragment:
			d = selectionDepth(v.SelectionSet, doc)
		case *ast.FragmentSpread:
			if frag := findFragment(doc, v.Name.Value); frag != nil {
				d = selectionDepth(frag.SelectionSet, doc)
			}
		}
		if d > max {
			max = d
		}
	}
	return max
}

// findFragment locates a named fragment definition in the document.
func findFragment(doc *ast.Document, name string) *ast.FragmentDefinition {
	for _, def := range doc.Definitions {
		if frag, ok := def.(*ast.FragmentDefinition); ok && frag.Name.Value == name {
			return frag
		}
	}
	return nil
}
