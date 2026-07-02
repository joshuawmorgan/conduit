package ast

import "strconv"

// unquoteRaw unquotes a raw double-quoted lexer token.
func unquoteRaw(raw string) string {
	if v, err := strconv.Unquote(raw); err == nil {
		return v
	}
	if len(raw) >= 2 && raw[0] == '"' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

// NameStr returns the workflow name without surrounding quotes.
func (w *Workflow) NameStr() string { return unquoteRaw(w.Name) }

// PathStr returns the import path without surrounding quotes.
func (i *Import) PathStr() string { return unquoteRaw(i.Path) }

// Attr returns the named attribute of a task, or nil.
func (t *Task) Attr(name string) *Value {
	for _, a := range t.Attrs {
		if a.Name == name {
			return a.Value
		}
	}
	return nil
}

// Attr returns the named workflow-level attribute value, or nil.
func (w *Workflow) Attr(name string) *Value {
	for _, a := range w.Attrs() {
		if a.Name == name {
			return a.Value
		}
	}
	return nil
}

// Unquote converts a raw double-quoted lexer token into its string value,
// falling back to trimming the outer quotes if it is not a valid Go literal.
func (s *String) Unquote() string {
	if s == nil {
		return ""
	}
	if v, err := strconv.Unquote(s.Value); err == nil {
		return v
	}
	if len(s.Value) >= 2 && s.Value[0] == '"' {
		return s.Value[1 : len(s.Value)-1]
	}
	return s.Value
}

// Key returns the normalized string form of a map key.
func (k *MapKey) Key() string {
	if k == nil {
		return ""
	}
	if k.Ident != nil {
		return *k.Ident
	}
	if k.Str != nil {
		return k.Str.Unquote()
	}
	return ""
}

// AsString returns the string content of a value when it is a string or bare
// identifier, plus ok=false for other kinds.
func (v *Value) AsString() (string, bool) {
	switch {
	case v == nil:
		return "", false
	case v.Str != nil:
		return v.Str.Unquote(), true
	case v.Ident != nil:
		return *v.Ident, true
	}
	return "", false
}

// AsIdentList returns the identifier/string items of a list value, used for
// attributes such as depends_on.
func (v *Value) AsIdentList() []string {
	if v == nil || v.List == nil {
		return nil
	}
	out := make([]string, 0, len(v.List.Items))
	for _, it := range v.List.Items {
		if s, ok := it.AsString(); ok {
			out = append(out, s)
		}
	}
	return out
}

// AsInt returns the integer value when the value is an integer literal.
func (v *Value) AsInt() (int64, bool) {
	if v != nil && v.Int != nil {
		return *v.Int, true
	}
	return 0, false
}

// AsMap flattens a Map value into a string->string map, stringifying scalars.
func (v *Value) AsMap() map[string]string {
	if v == nil || v.Map == nil {
		return nil
	}
	out := make(map[string]string, len(v.Map.Entries))
	for _, e := range v.Map.Entries {
		out[e.Key.Key()] = e.Value.Stringify()
	}
	return out
}

// Native converts a value to its Go representation.
func (v *Value) Native() any {
	switch {
	case v == nil:
		return nil
	case v.Str != nil:
		return v.Str.Unquote()
	case v.Float != nil:
		return *v.Float
	case v.Int != nil:
		return *v.Int
	case v.Bool != nil:
		return v.Bool.Bool()
	case v.Ident != nil:
		return *v.Ident
	case v.List != nil:
		items := make([]any, 0, len(v.List.Items))
		for _, it := range v.List.Items {
			items = append(items, it.Native())
		}
		return items
	case v.Map != nil:
		m := make(map[string]any, len(v.Map.Entries))
		for _, e := range v.Map.Entries {
			m[e.Key.Key()] = e.Value.Native()
		}
		return m
	}
	return nil
}

// Stringify renders a scalar value as a string for use in string-typed contexts.
func (v *Value) Stringify() string {
	switch {
	case v == nil:
		return ""
	case v.Str != nil:
		return v.Str.Unquote()
	case v.Float != nil:
		return strconv.FormatFloat(*v.Float, 'g', -1, 64)
	case v.Int != nil:
		return strconv.FormatInt(*v.Int, 10)
	case v.Bool != nil:
		return strconv.FormatBool(v.Bool.Bool())
	case v.Ident != nil:
		return *v.Ident
	}
	return ""
}
