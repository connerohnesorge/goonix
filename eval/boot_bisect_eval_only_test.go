package eval
import ("path/filepath"; "testing"; "github.com/connerohnesorge/goonix/parser")
func TestBootBisectEvalExprOnly(t *testing.T) {
	root := testModRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "boot_chain_bisect_probe.nix")
	abs, _ := filepath.Abs(path)
	src := `import "` + abs + `" {}`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + filepath.Join(root, "testdata", "nixpkgs"))
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, _ := parser.ParseString(src)
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	_, err := ev.evalExpr(expr, env)
	if err != nil { t.Fatalf("evalExpr: %v", err) }
}

func TestBootDepth0CcTypeOnly(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + nixpkgs + `/default.nix" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in (builtins.tryEval (builtins.typeOf pkgs.stdenv.cc)).success`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, _ := parser.ParseString(src)
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, _ := ev.forceValue(v)
	t.Logf("ccType tryEval success=%v", fv.Bool)
}

func TestBootDepth0HasOvOnly(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + nixpkgs + `/default.nix" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in (builtins.tryEval (pkgs.stdenv ? overrides)).success`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, _ := parser.ParseString(src)
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, _ := ev.forceValue(v)
	t.Logf("hasOv tryEval success=%v", fv.Bool)
}

func TestBootDepth0TryStdenv(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + nixpkgs + `/default.nix" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in (builtins.tryEval pkgs.stdenv).success`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, _ := parser.ParseString(src)
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, _ := ev.forceValue(v)
	t.Logf("tryEval stdenv success=%v", fv.Bool)
}

func TestBootDepth0TryStdenvMsg(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + nixpkgs + `/default.nix" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
    r = builtins.tryEval pkgs.stdenv;
in if r.success then "ok" else r.value`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, _ := parser.ParseString(src)
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, _ := ev.forceValue(v)
	t.Logf("msg=%s", fv.String)
}

func TestBootForceGccBeforeStdenv(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + nixpkgs + `/default.nix" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in { g = (builtins.tryEval pkgs.gccStdenv).success; s = (builtins.tryEval pkgs.stdenv).success; }`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, _ := parser.ParseString(src)
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, _ := ev.forceValue(v)
	for sym, b := range fv.Attrs.Binds {
		name := ev.Intern.Lookup(sym)
		val, _ := ev.forceValue(b.Value)
		t.Logf("%s=%v", name, val.Bool)
	}
}
