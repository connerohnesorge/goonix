// Copyright 2026 The goonix authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "parse":
		cmdParse(os.Args[2:])
	case "eval":
		cmdEval(os.Args[2:])
	case "instantiate":
		cmdInstantiate(os.Args[2:])
	case "derivation-show":
		cmdDerivationShow(os.Args[2:])
	case "build":
		cmdBuild(os.Args[2:])
	case "drv-diff":
		cmdDrvDiff(os.Args[2:])
	case "gc":
		cmdGC(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `goonix — minimal Nix evaluator

Usage:
  goonix parse <file.nix>
  goonix eval [--json] [--pure] [-I path] <file.nix>
  goonix instantiate [--store DIR] <file.nix>
  goonix build [--store DIR] [-I path] <file.nix>
  goonix derivation-show [--go-eval FILE [-I path]...] [--nix PATH]
  goonix drv-diff --nix-instantiate FILE | --nix-drv PATH --go-eval FILE [-I path]...
  goonix gc --store DIR

`)
}

func cmdParse(args []string) {
	fs := flag.NewFlagSet("parse", flag.ExitOnError)
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "parse expects one file argument")
		os.Exit(2)
	}
	expr, err := parser.ParseFile(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(parser.Print(expr))
}

func cmdEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit JSON")
	pure := fs.Bool("pure", true, "pure evaluation mode")
	searchPath := fs.String("I", "", "add search path entry")
	extraXP := fs.String("extra-experimental-features", "", "space-separated experimental features (flakes enables fetch-tree)")
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "eval expects one file argument")
		os.Exit(2)
	}
	ev := eval.NewEvaluator()
	ev.SetCurFile(rest[0])
	if !*pure {
		ev = ev.WithEffects(eval.IOEffects{})
	}
	ev = ev.WithExperimental(strings.Fields(*extraXP)...)
	if *searchPath != "" {
		for _, part := range strings.Split(*searchPath, ":") {
			part = strings.TrimSpace(part)
			if part != "" {
				ev.NixPath.PrependEntries(part)
			}
		}
	}
	expr, err := parser.ParseFile(rest[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *jsonOut {
		d := eval.ToData(val, ev.Intern)
		b, _ := json.MarshalIndent(d, "", "  ")
		fmt.Println(string(b))
		return
	}
	fmt.Println(val.DisplayString(ev, ev.Intern))
}

func cmdInstantiate(args []string) {
	fs := flag.NewFlagSet("instantiate", flag.ExitOnError)
	storeDir := fs.String("store", store.DefaultStoreDir, "store directory")
	_ = fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "instantiate expects one file argument")
		os.Exit(2)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{Root: *storeDir})
	ev.SetCurFile(rest[0])
	expr, err := parser.ParseFile(rest[0])
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
	drv, err := ev.AttrsToDerivation(fv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cfg := store.Config{StoreDir: *storeDir}
	if err := ev.WriteDerivationClosure(cfg, drv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(strings.TrimSpace(drv.DrvPath))
}
