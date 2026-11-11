package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/builder"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
	"github.com/connerohnesorge/goonix/substituter"
)

func cmdBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	storeFlag := fs.String("store", "", "physical store root (contains store/ subdirectory)")
	extraXP := fs.String("extra-experimental-features", "", "space-separated experimental features (flakes enables fetch-tree)")
	allowRelocated := fs.Bool("allow-relocated-store", false, "allow physical store path rewriting when /nix/store bind mount is unavailable (testing only)")
	ra, flags, err := splitRootArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	_ = fs.Parse(flags)
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "build expects one .nix file or directory")
		os.Exit(2)
	}
	physRoot := *storeFlag
	if physRoot == "" {
		physRoot = filepath.Join(os.TempDir(), "goonix-build-store")
	}
	st, err := local.OpenStore(physRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{Root: st.Config().LogicalStoreDir()}).WithExperimental(strings.Fields(*extraXP)...)
	ra.apply(ev)
	expr, curFile, err := ra.expr(rest[0])
	ev.SetCurFile(curFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		drvPath, err = eval.ForceDerivationAttrs(ev, fv)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	gd, ok := ev.RegisteredDerivation(drvPath)
	if !ok {
		fmt.Fprintln(os.Stderr, "derivation was not registered during eval")
		os.Exit(1)
	}
	if err := writeRegisteredDrvClosure(st, ev, drvPath, map[string]bool{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sub := substituter.NewClient(substituter.Config{})
	b := &builder.Builder{
		Store: st,
		Config: builder.Config{
			Substituter:    sub,
			AllowRelocated: *allowRelocated,
		},
		Load: func(p string) (*gonixdrv.Derivation, error) {
			if d, ok := ev.RegisteredDerivation(p); ok {
				return d, nil
			}
			f, err := os.Open(local.PhysicalPath(st.Root, p))
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return gonixdrv.ReadDerivation(f)
		},
	}
	if err := b.BuildRealisation(drvPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	outPaths, err := store.LogicalOutputPaths(gd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for name, p := range outPaths {
		p = st.Config().RewriteDefaultStoreDir(p)
		fmt.Printf("%s=%s\n", name, p)
	}
	_ = store.DefaultStoreDir
}

func writeRegisteredDrvClosure(st *local.Store, ev *eval.Evaluator, drvPath string, seen map[string]bool) error {
	if seen[drvPath] {
		return nil
	}
	seen[drvPath] = true
	gd, ok := ev.RegisteredDerivation(drvPath)
	if !ok {
		return fmt.Errorf("derivation %s was not registered during eval", drvPath)
	}
	for dep := range gd.InputDerivations {
		if err := writeRegisteredDrvClosure(st, ev, dep, seen); err != nil {
			return err
		}
	}
	aterm, err := eval.SerializeGoNixDerivation(gd)
	if err != nil {
		return err
	}
	return st.WriteDerivation(aterm, drvPath)
}
