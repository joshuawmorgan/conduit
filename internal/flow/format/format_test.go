package format

import (
	"testing"

	"github.com/conduit-io/conduit/internal/flow/parser"
)

func TestRoundTripIsStable(t *testing.T) {
	src := `workflow "w" {
  description: "demo"
  param name: string = "world"

  task greet {
    run: "echo hi ${params.name}"
    retry: 2
  }

  task done {
    depends_on: [greet]
    uses: "builtin/print"
    with: {
      message: "bye"
    }
  }
}
`
	f1, err := parser.ParseString("t.flow", src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out1 := File(f1)

	// Formatting the formatted output must be a fixed point.
	f2, err := parser.ParseString("t.flow", out1)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, out1)
	}
	out2 := File(f2)
	if out1 != out2 {
		t.Fatalf("format not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out1, out2)
	}
}

func TestBoolDefaultFormatsCorrectly(t *testing.T) {
	// Regression: Participle bool capture must not coerce false -> true.
	f, err := parser.ParseString("t.flow", `workflow "w" { param b: bool = false task a { run: "x" } }`)
	if err != nil {
		t.Fatal(err)
	}
	out := File(f)
	if want := "param b: bool = false"; !contains(out, want) {
		t.Fatalf("expected %q in:\n%s", want, out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
