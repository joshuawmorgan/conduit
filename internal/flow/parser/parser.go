// Package parser builds FlowDSL ASTs from source text using Participle v2.
//
// The lexer elides whitespace, comments and commas so the grammar in the ast
// package stays small and forgiving. Diagnostics are surfaced as *ParseError
// carrying a source position for the LSP and CLI.
package parser

import (
	"fmt"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"

	"github.com/conduit-io/conduit/internal/flow/ast"
)

// flowLexer defines the FlowDSL token set. Order matters: earlier rules win.
var flowLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Comment", Pattern: `//[^\n]*|#[^\n]*`},
	{Name: "Whitespace", Pattern: `[ \t\r\n]+`},
	{Name: "Float", Pattern: `[-+]?[0-9]+\.[0-9]+`},
	{Name: "Int", Pattern: `[-+]?[0-9]+`},
	{Name: "String", Pattern: `"(\\.|[^"\\])*"`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "Comma", Pattern: `,`},
	{Name: "Punct", Pattern: `[{}\[\]:=]`},
})

// parser is the compiled, reusable FlowDSL parser.
var parser = participle.MustBuild[ast.File](
	participle.Lexer(flowLexer),
	participle.Elide("Comment", "Whitespace", "Comma"),
	participle.UseLookahead(2),
)

// ParseError is a positioned parse diagnostic.
type ParseError struct {
	Position lexer.Position
	Message  string
}

func (e *ParseError) Error() string {
	if e.Position.Filename != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.Position.Filename, e.Position.Line, e.Position.Column, e.Message)
	}
	return fmt.Sprintf("%d:%d: %s", e.Position.Line, e.Position.Column, e.Message)
}

// ParseString parses FlowDSL source, tagging positions with filename.
func ParseString(filename, source string) (*ast.File, error) {
	f, err := parser.ParseString(filename, source)
	if err != nil {
		return nil, toParseError(err)
	}
	return f, nil
}

// ParseBytes parses FlowDSL from a byte slice.
func ParseBytes(filename string, data []byte) (*ast.File, error) {
	return ParseString(filename, string(data))
}

func toParseError(err error) error {
	var perr participle.Error
	if pe, ok := err.(participle.Error); ok {
		perr = pe
		return &ParseError{Position: perr.Position(), Message: perr.Message()}
	}
	return &ParseError{Message: err.Error()}
}

// Grammar returns the EBNF-like description of the FlowDSL grammar, used by
// `conduit docs` and diagnostics.
func Grammar() string {
	return parser.String()
}
