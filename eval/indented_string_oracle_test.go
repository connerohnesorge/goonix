package eval

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestUnescapeIndentedStringDollarBrace(t *testing.T) {
	got := unescapeIndentedString(`''${foo}`)
	if got != "${foo}" {
		t.Fatalf("got %q", got)
	}
}

func TestIndentedStringInterpolationAfterTextMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let tools = "/nix/store/abc-bootstrap-tools"; in ''
    mkdir -p $out
    ln -s ${tools}/lib $out/lib
  ''`
	want := lay.EvalStdinRaw(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ev.forceString(fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIndentedStringStripAndUnescapeMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let s = ''
    ''${x}-ok
  ''; in s`
	want := lay.EvalStdinRaw(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ev.forceString(fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIndentedStringTrailingNewlineMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let s = ''
    line-one
    line-two
  ''; in s`
	want := lay.EvalStdinRaw(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ev.forceString(fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIndentedStringFirstFragmentTrailingNewlineMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let tools = "/nix/store/abc-bootstrap-tools"; a = ''
    mkdir -p $out
    ln -s ${tools}/lib $out/lib
  ''; in a`
	want := lay.EvalStdinRaw(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ev.forceString(fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIndentedStringSecondFragmentTrailingNewlineMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let tools = "/nix/store/abc-bootstrap-tools"; b = ''
    ln -s ${tools}/include-glibc $out/include
  ''; in b`
	want := lay.EvalStdinRaw(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ev.forceString(fv)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("second fragment ends with newline: want=%v got=%v", strings.HasSuffix(want, "\n"), strings.HasSuffix(got, "\n"))
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestIndentedStringConcatTwoFragmentsMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let tools = "/nix/store/abc-bootstrap-tools";
  a = ''
    mkdir -p $out
    ln -s ${tools}/lib $out/lib
  '';
  b = ''
    ln -s ${tools}/include-glibc $out/include
  '';
in a + b`
	want := lay.EvalStdinRaw(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ev.forceString(fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
