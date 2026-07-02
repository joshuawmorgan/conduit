package cel

import "testing"

func TestEvalBool(t *testing.T) {
	e, err := New()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		expr string
		act  Activation
		want bool
	}{
		{`params.branch == "main"`, Activation{Params: map[string]any{"branch": "main"}}, true},
		{`params.branch == "main"`, Activation{Params: map[string]any{"branch": "dev"}}, false},
		{`!params.skip`, Activation{Params: map[string]any{"skip": false}}, true},
		{`params.n > 3`, Activation{Params: map[string]any{"n": int64(5)}}, true},
		{``, Activation{}, true}, // empty guard defaults to true
	}
	for _, c := range cases {
		got, err := e.EvalBool(c.expr, c.act)
		if err != nil {
			t.Errorf("%q: %v", c.expr, err)
			continue
		}
		if got != c.want {
			t.Errorf("%q = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestInterpolate(t *testing.T) {
	e, _ := New()
	act := Activation{
		Params: map[string]any{"name": "world", "n": int64(2)},
		Env:    map[string]string{"HOME": "/home/u"},
	}
	cases := map[string]string{
		"hello ${params.name}":          "hello world",
		"count=${params.n + 1}":         "count=3",
		"home ${env.HOME}":              "home /home/u",
		"literal $${not.evaluated}":     "literal ${not.evaluated}",
		"nested ${params.name + \"!\"}": "nested world!",
	}
	for in, want := range cases {
		got, err := e.Interpolate(in, act)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Interpolate(%q) = %q, want %q", in, got, want)
		}
	}
}
