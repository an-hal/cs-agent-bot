package collection

import (
	"testing"
)

func TestAsFloat_AllNumericKinds(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want float64
		ok   bool
	}{
		{name: "float64", in: float64(3.14), want: 3.14, ok: true},
		{name: "float32", in: float32(1.5), want: 1.5, ok: true},
		{name: "int", in: 42, want: 42, ok: true},
		{name: "int64", in: int64(7), want: 7, ok: true},
		{name: "numeric string", in: "2.5", want: 2.5, ok: true},
		{name: "non-numeric string", in: "banana", ok: false},
		{name: "bool rejected", in: true, ok: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := asFloat(tc.in)
			if ok != tc.ok {
				t.Fatalf("ok: got %v want %v", ok, tc.ok)
			}
			if ok && got != tc.want {
				t.Fatalf("val: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestUnquote(t *testing.T) {
	cases := map[string]string{
		`"hello"`: "hello",
		`hello`:   "hello",
		`""`:      "",
		`"a`:      `"a`,
	}
	for in, want := range cases {
		if got := unquote(in); got != want {
			t.Fatalf("unquote(%q): got %q want %q", in, got, want)
		}
	}
}

func TestFieldErrsToSlice_Ordered(t *testing.T) {
	m := map[string]string{
		"zebra": "zed",
		"alpha": "aah",
	}
	got := fieldErrsToSlice(m)
	if len(got) != 2 {
		t.Fatalf("len: got %d want 2", len(got))
	}
	if got[0].Field != "alpha" || got[1].Field != "zebra" {
		t.Fatalf("not sorted: %+v", got)
	}
}

func TestFieldErrsToSlice_Nil(t *testing.T) {
	if fieldErrsToSlice(nil) != nil {
		t.Fatal("expected nil on empty input")
	}
}
