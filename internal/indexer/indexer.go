package indexer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type Indexer struct {
	pyParser *sitter.Parser
	tsParser *sitter.Parser
}

func NewIndexer() *Indexer {
	pyParser := sitter.NewParser()
	pyParser.SetLanguage(python.GetLanguage())

	tsParser := sitter.NewParser()
	tsParser.SetLanguage(typescript.GetLanguage())

	return &Indexer{
		pyParser: pyParser,
		tsParser: tsParser,
	}
}

// ParseContent extracts symbols and dependencies from the source code based on its extension
func (i *Indexer) ParseContent(ctx context.Context, path string, content []byte) ([]Symbol, []Edge, error) {
	ext := strings.ToLower(filepath.Ext(path))
	
	switch ext {
	case ".py":
		return i.parsePython(ctx, content)
	case ".ts", ".js":
		return i.parseTypeScript(ctx, content)
	default:
		// Unsupported language, return empty
		return nil, nil, nil
	}
}

func (i *Indexer) parsePython(ctx context.Context, content []byte) ([]Symbol, []Edge, error) {
	tree, err := i.pyParser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse python: %w", err)
	}
	defer tree.Close()

	var symbols []Symbol
	var edges []Edge

	// Basic traversal to find functions and classes
	n := tree.RootNode()
	traverseAndCollect(n, content, &symbols)

	return symbols, edges, nil
}

func (i *Indexer) parseTypeScript(ctx context.Context, content []byte) ([]Symbol, []Edge, error) {
	tree, err := i.tsParser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse typescript: %w", err)
	}
	defer tree.Close()

	var symbols []Symbol
	var edges []Edge

	// Basic traversal to find functions and classes
	n := tree.RootNode()
	traverseAndCollect(n, content, &symbols)

	return symbols, edges, nil
}

// traverseAndCollect does a naive DFS to pull out function and class definitions.
func traverseAndCollect(node *sitter.Node, content []byte, symbols *[]Symbol) {
	if node == nil {
		return
	}

	nodeType := node.Type()
	
	// A naive extraction based on tree-sitter node types
	if nodeType == "function_definition" || nodeType == "function_declaration" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			*symbols = append(*symbols, Symbol{
				Name:      nameNode.Content(content),
				Type:      SymbolTypeFunction,
				StartLine: node.StartPoint().Row + 1,
				EndLine:   node.EndPoint().Row + 1,
			})
		}
	} else if nodeType == "class_definition" || nodeType == "class_declaration" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			*symbols = append(*symbols, Symbol{
				Name:      nameNode.Content(content),
				Type:      SymbolTypeClass,
				StartLine: node.StartPoint().Row + 1,
				EndLine:   node.EndPoint().Row + 1,
			})
		}
	} else if nodeType == "method_definition" {
		nameNode := node.ChildByFieldName("name")
		if nameNode != nil {
			*symbols = append(*symbols, Symbol{
				Name:      nameNode.Content(content),
				Type:      SymbolTypeMethod,
				StartLine: node.StartPoint().Row + 1,
				EndLine:   node.EndPoint().Row + 1,
			})
		}
	} else if nodeType == "import_statement" || nodeType == "import_from_statement" {
		// Basic python import capture
		// For milestone 1 we just capture the fact that this file has an import
		// Real edges require a multi-pass linker
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		traverseAndCollect(node.Child(i), content, symbols)
	}
}
