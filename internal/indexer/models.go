package indexer

import "fmt"

type SymbolType string

const (
	SymbolTypeFunction SymbolType = "function"
	SymbolTypeClass    SymbolType = "class"
	SymbolTypeMethod   SymbolType = "method"
	SymbolTypeVariable SymbolType = "variable"
)

type Symbol struct {
	Name      string
	Type      SymbolType
	StartLine uint32
	EndLine   uint32
	Context   string // snippet of the symbol signature or definition
}

type EdgeType string

const (
	EdgeTypeDependsOn   EdgeType = "depends_on"
	EdgeTypeImplements  EdgeType = "implements"
	EdgeTypeTestedBy    EdgeType = "tested_by"
)

type Edge struct {
	SourceNode string
	TargetNode string
	Type       EdgeType
}

func (s Symbol) String() string {
	return fmt.Sprintf("%s:%s (lines %d-%d)", s.Type, s.Name, s.StartLine, s.EndLine)
}
