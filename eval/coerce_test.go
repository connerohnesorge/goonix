package eval

import (
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestCoerceAttrSetOutPath(t *testing.T) {
	ev := NewEvaluator()
	outPath := StrWithContext("/nix/store/abc-out", []StringContextElem{
		{DrvPath: "/nix/store/abc.drv", Output: "out"},
	}, parser.Pos{})
	attrs := Attrs(false)
	attrs.Attrs.Binds[ev.Intern.Intern("outPath")] = &AttrBinding{Value: outPath}
	sv, err := ev.coerceToString(attrs, CoerceContext{Mode: CoerceString})
	if err != nil {
		t.Fatal(err)
	}
	if sv.String != "/nix/store/abc-out" {
		t.Fatalf("got %q", sv.String)
	}
	if len(sv.Ctx) != 1 || sv.Ctx[0].DrvPath == "" {
		t.Fatalf("ctx %v", sv.Ctx)
	}
}

func TestCoerceAttrSetWithoutOutPathErrors(t *testing.T) {
	ev := NewEvaluator()
	attrs := Attrs(false)
	attrs.Attrs.Binds[ev.Intern.Intern("x")] = &AttrBinding{Value: Int(1)}
	_, err := ev.coerceToString(attrs, CoerceContext{Mode: CoerceString})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPathPlusStringStaysPath(t *testing.T) {
	ev := NewEvaluator()
	p := Path("./foo")
	s := Str("bar", parser.Pos{})
	v, err := ev.concatCoercedStrings(p, s, parser.Pos{})
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != VKPath || v.Path != "./foobar" {
		t.Fatalf("got %v", v)
	}
	if len(v.Ctx) != 0 {
		t.Fatalf("unexpected ctx %v", v.Ctx)
	}
}
