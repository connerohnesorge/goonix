package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/nix-community/go-nix/pkg/narinfo/signature"

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
	substituterURL := fs.String("substituter", "", "binary cache URL (default https://cache.nixos.org)")
	var trustedKeys stringList
	fs.Var(&trustedKeys, "trusted-key", "public key NAME:BASE64 that signs the substituter's narinfo (repeatable)")
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
		physRoot = defaultBuildStoreRoot()
	}
	st, err := local.OpenStore(physRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer st.Close()

	// Files the evaluation adds to the store (builder scripts, toFile, path
	// coercions) go to the physical store the builder reads from.
	ev := eval.NewEvaluator().WithEffects(eval.ChrootIOEffects{PhysRoot: physRoot}).WithExperimental(strings.Fields(*extraXP)...)
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

	subCfg := substituter.Config{CacheURL: *substituterURL}
	for _, k := range trustedKeys {
		pk, err := signature.ParsePublicKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --trusted-key %q: %v\n", k, err)
			os.Exit(2)
		}
		subCfg.TrustedKeys = append(subCfg.TrustedKeys, pk)
	}
	sub := substituter.NewClient(subCfg)
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
	names := make([]string, 0, len(outPaths))
	for name := range outPaths {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		p := st.Config().RewriteDefaultStoreDir(outPaths[name])
		// On Windows the result is printed where it is on disk, the path a
		// user runs, as NovaNix prints it.
		if runtime.GOOS == "windows" {
			p = local.PhysicalPath(st.Root, p)
		}
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

// stringList is a repeatable string flag.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, " ") }

func (l *stringList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// defaultBuildStoreRoot is the physical store root build uses without
// --store. On Windows it is C:\nix, so the store is C:\nix\store, NovaNix's
// default and the directory its binary cache's outputs were built in.
func defaultBuildStoreRoot() string {
	if runtime.GOOS == "windows" {
		return `C:\nix`
	}
	return filepath.Join(os.TempDir(), "goonix-build-store")
}
