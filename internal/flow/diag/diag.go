// Package diag defines the diagnostic model shared by the parser, semantic
// analyzer, linter and LSP server.
package diag

import (
	"fmt"
	"sort"

	"github.com/alecthomas/participle/v2/lexer"
)

// Severity classifies a diagnostic.
type Severity int

const (
	Error Severity = iota
	Warning
	Information
	Hint
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Information:
		return "info"
	case Hint:
		return "hint"
	default:
		return "unknown"
	}
}

// Diagnostic is a single positioned finding. Code follows the FLOW-Exxxx /
// FLOW-Wxxxx registry described in docs/24-semantic-analysis.md.
type Diagnostic struct {
	Severity Severity       `json:"severity"`
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Pos      lexer.Position `json:"position"`
}

func (d Diagnostic) String() string {
	return fmt.Sprintf("%s:%d:%d: %s [%s] %s",
		d.Pos.Filename, d.Pos.Line, d.Pos.Column, d.Severity, d.Code, d.Message)
}

// Diagnostics is an ordered, appendable collection.
type Diagnostics struct {
	Items []Diagnostic
}

// Add appends a diagnostic.
func (d *Diagnostics) Add(sev Severity, code, msg string, pos lexer.Position) {
	d.Items = append(d.Items, Diagnostic{Severity: sev, Code: code, Message: msg, Pos: pos})
}

// Errorf appends an error diagnostic.
func (d *Diagnostics) Errorf(code string, pos lexer.Position, format string, args ...any) {
	d.Add(Error, code, fmt.Sprintf(format, args...), pos)
}

// Warnf appends a warning diagnostic.
func (d *Diagnostics) Warnf(code string, pos lexer.Position, format string, args ...any) {
	d.Add(Warning, code, fmt.Sprintf(format, args...), pos)
}

// HasErrors reports whether any diagnostic is an error.
func (d *Diagnostics) HasErrors() bool {
	for _, it := range d.Items {
		if it.Severity == Error {
			return true
		}
	}
	return false
}

// Sort orders diagnostics by position then severity for stable output.
func (d *Diagnostics) Sort() {
	sort.SliceStable(d.Items, func(i, j int) bool {
		a, b := d.Items[i], d.Items[j]
		if a.Pos.Line != b.Pos.Line {
			return a.Pos.Line < b.Pos.Line
		}
		if a.Pos.Column != b.Pos.Column {
			return a.Pos.Column < b.Pos.Column
		}
		return a.Severity < b.Severity
	})
}

// Len returns the number of diagnostics.
func (d *Diagnostics) Len() int { return len(d.Items) }

// ErrorCount returns the number of error-severity diagnostics.
func (d *Diagnostics) ErrorCount() int {
	n := 0
	for _, it := range d.Items {
		if it.Severity == Error {
			n++
		}
	}
	return n
}
