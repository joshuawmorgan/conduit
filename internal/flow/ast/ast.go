// Package ast defines the FlowDSL abstract syntax tree. The grammar is driven
// by github.com/alecthomas/participle/v2 struct tags. The concrete syntax is a
// clean, regular block language:
//
//	workflow "ci" {
//	  description: "Build and test"
//	  param branch: string = "main"
//	  env: { GOFLAGS: "-mod=mod" }
//
//	  task build {
//	    run: "go build ./..."
//	    timeout: "5m"
//	  }
//	  task test {
//	    depends_on: [build]
//	    when: "params.branch == \"main\""
//	    run: "go test ./..."
//	    retry: 2
//	  }
//	}
//
// Scalar attributes use `key: value`. Blocks use `keyword label { ... }`.
// Expressions (`when`) and string interpolation (`${ ... }`) are opaque to the
// grammar and evaluated at runtime by the CEL engine.
package ast

import "github.com/alecthomas/participle/v2/lexer"

// File is the top-level unit: zero or more imports and workflows.
type File struct {
	Pos     lexer.Position
	Entries []*Entry `parser:"@@*"`
}

// Workflows returns just the workflow declarations in source order.
func (f *File) Workflows() []*Workflow {
	var out []*Workflow
	for _, e := range f.Entries {
		if e.Workflow != nil {
			out = append(out, e.Workflow)
		}
	}
	return out
}

// Imports returns just the import declarations.
func (f *File) Imports() []*Import {
	var out []*Import
	for _, e := range f.Entries {
		if e.Import != nil {
			out = append(out, e.Import)
		}
	}
	return out
}

// Entry is a top-level file item.
type Entry struct {
	Pos      lexer.Position
	Import   *Import   `parser:"  @@"`
	Workflow *Workflow `parser:"| @@"`
}

// Import pulls definitions from another .flow file.
type Import struct {
	Pos  lexer.Position
	Path string `parser:"'import' @String"`
}

// Workflow is a named collection of tasks plus workflow-level attributes.
type Workflow struct {
	Pos   lexer.Position
	Name  string          `parser:"'workflow' @String '{'"`
	Items []*WorkflowItem `parser:"@@* '}'"`
}

// Params returns the workflow parameter declarations.
func (w *Workflow) Params() []*Param {
	var out []*Param
	for _, it := range w.Items {
		if it.Param != nil {
			out = append(out, it.Param)
		}
	}
	return out
}

// Tasks returns the task declarations.
func (w *Workflow) Tasks() []*Task {
	var out []*Task
	for _, it := range w.Items {
		if it.Task != nil {
			out = append(out, it.Task)
		}
	}
	return out
}

// Attrs returns workflow-level scalar attributes (description, env, ...).
func (w *Workflow) Attrs() []*Attr {
	var out []*Attr
	for _, it := range w.Items {
		if it.Attr != nil {
			out = append(out, it.Attr)
		}
	}
	return out
}

// WorkflowItem is one of: param, task, or a scalar attribute.
type WorkflowItem struct {
	Pos   lexer.Position
	Param *Param `parser:"  @@"`
	Task  *Task  `parser:"| @@"`
	Attr  *Attr  `parser:"| @@"`
}

// Param declares a typed workflow input with an optional default.
type Param struct {
	Pos     lexer.Position
	Name    string `parser:"'param' @Ident"`
	Type    string `parser:"':' @Ident"`
	Default *Value `parser:"('=' @@)?"`
}

// Task is a unit of work in the DAG.
type Task struct {
	Pos   lexer.Position
	Name  string  `parser:"'task' @Ident '{'"`
	Attrs []*Attr `parser:"@@* '}'"`
}

// Attr is a `key: value` assignment.
type Attr struct {
	Pos   lexer.Position
	Name  string `parser:"@Ident ':'"`
	Value *Value `parser:"@@"`
}

// Value is a scalar, collection, or bare identifier.
type Value struct {
	Pos   lexer.Position
	Str   *String  `parser:"  @@"`
	Float *float64 `parser:"| @Float"`
	Int   *int64   `parser:"| @Int"`
	Bool  *Bool    `parser:"| @@"`
	List  *List    `parser:"| @@"`
	Map   *Map     `parser:"| @@"`
	Ident *string  `parser:"| @Ident"`
}

// String wraps a quoted string literal so it can be unquoted on capture.
type String struct {
	Value string `parser:"@String"`
}

// Bool captures true/false keywords. The raw token text is stored because
// Participle sets a bool field to true whenever the group matches; we convert
// explicitly via Bool().
type Bool struct {
	Raw string `parser:"@('true' | 'false')"`
}

// Bool returns the boolean value of the literal.
func (b *Bool) Bool() bool { return b != nil && b.Raw == "true" }

// List is a bracketed sequence of values. Separating commas are elided by the
// lexer, so both `[a, b]` and `[a b]` parse.
type List struct {
	Pos   lexer.Position
	Items []*Value `parser:"'[' @@* ']'"`
}

// Map is a brace-delimited set of key/value entries.
type Map struct {
	Pos     lexer.Position
	Entries []*MapEntry `parser:"'{' @@* '}'"`
}

// MapEntry is one `key: value` pair inside a Map. Keys may be identifiers or
// quoted strings.
type MapEntry struct {
	Pos   lexer.Position
	Key   *MapKey `parser:"@@ ':'"`
	Value *Value  `parser:"@@"`
}

// MapKey is an identifier or string used as a map key.
type MapKey struct {
	Ident *string `parser:"  @Ident"`
	Str   *String `parser:"| @@"`
}
