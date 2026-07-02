// Package format renders a FlowDSL AST back to canonical source text. It is
// used by `conduit fmt` and by round-trip tests.
package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/conduit-io/conduit/internal/flow/ast"
)

const indentUnit = "  "

// File renders a whole file to canonical FlowDSL.
func File(f *ast.File) string {
	var b strings.Builder
	imports := f.Imports()
	for _, im := range imports {
		fmt.Fprintf(&b, "import %s\n", quote(im.PathStr()))
	}
	if len(imports) > 0 && len(f.Workflows()) > 0 {
		b.WriteString("\n")
	}
	for i, wf := range f.Workflows() {
		if i > 0 {
			b.WriteString("\n")
		}
		writeWorkflow(&b, wf)
	}
	return b.String()
}

func writeWorkflow(b *strings.Builder, wf *ast.Workflow) {
	fmt.Fprintf(b, "workflow %s {\n", quote(wf.NameStr()))
	for _, a := range wf.Attrs() {
		writeAttr(b, 1, a)
	}
	for _, p := range wf.Params() {
		writeIndent(b, 1)
		fmt.Fprintf(b, "param %s: %s", p.Name, p.Type)
		if p.Default != nil {
			b.WriteString(" = ")
			writeValue(b, 1, p.Default)
		}
		b.WriteString("\n")
	}
	for _, t := range wf.Tasks() {
		b.WriteString("\n")
		writeTask(b, t)
	}
	b.WriteString("}\n")
}

func writeTask(b *strings.Builder, t *ast.Task) {
	writeIndent(b, 1)
	fmt.Fprintf(b, "task %s {\n", t.Name)
	for _, a := range t.Attrs {
		writeAttr(b, 2, a)
	}
	writeIndent(b, 1)
	b.WriteString("}\n")
}

func writeAttr(b *strings.Builder, depth int, a *ast.Attr) {
	writeIndent(b, depth)
	fmt.Fprintf(b, "%s: ", a.Name)
	writeValue(b, depth, a.Value)
	b.WriteString("\n")
}

func writeValue(b *strings.Builder, depth int, v *ast.Value) {
	switch {
	case v == nil:
		b.WriteString("null")
	case v.Str != nil:
		b.WriteString(quote(v.Str.Unquote()))
	case v.Float != nil:
		b.WriteString(strconv.FormatFloat(*v.Float, 'g', -1, 64))
	case v.Int != nil:
		b.WriteString(strconv.FormatInt(*v.Int, 10))
	case v.Bool != nil:
		b.WriteString(strconv.FormatBool(v.Bool.Bool()))
	case v.Ident != nil:
		b.WriteString(*v.Ident)
	case v.List != nil:
		b.WriteString("[")
		for i, it := range v.List.Items {
			if i > 0 {
				b.WriteString(", ")
			}
			writeValue(b, depth, it)
		}
		b.WriteString("]")
	case v.Map != nil:
		if len(v.Map.Entries) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for _, e := range v.Map.Entries {
			writeIndent(b, depth+1)
			fmt.Fprintf(b, "%s: ", e.Key.Key())
			writeValue(b, depth+1, e.Value)
			b.WriteString("\n")
		}
		writeIndent(b, depth)
		b.WriteString("}")
	}
}

func writeIndent(b *strings.Builder, depth int) {
	for i := 0; i < depth; i++ {
		b.WriteString(indentUnit)
	}
}

func quote(s string) string { return strconv.Quote(s) }
