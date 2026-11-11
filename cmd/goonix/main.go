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
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

// startProfile writes a CPU profile to $GOONIX_CPUPROFILE. With
// $GOONIX_PROFILE_SECONDS it stops after that many seconds, writes a heap
// profile to $GOONIX_HEAPPROFILE if set, and exits, so a run that never
// finishes still leaves profiles.
func startProfile() func() {
	path := os.Getenv("GOONIX_CPUPROFILE")
	if path == "" {
		return func() {}
	}
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	stop := func() { pprof.StopCPUProfile(); f.Close() }
	if secs, err := strconv.Atoi(os.Getenv("GOONIX_PROFILE_SECONDS")); err == nil && secs > 0 {
		go func() {
			time.Sleep(time.Duration(secs) * time.Second)
			stop()
			eval.RequestStatsDump()
			if hp := os.Getenv("GOONIX_HEAPPROFILE"); hp != "" {
				if hf, err := os.Create(hp); err == nil {
					_ = pprof.WriteHeapProfile(hf)
					hf.Close()
				}
			}
			if os.Getenv("GOONIX_EVAL_STATS") != "" {
				// Give the evaluator time to print its counters and exit.
				time.Sleep(10 * time.Second)
			}
			fmt.Fprintln(os.Stderr, "goonix: profile time limit reached")
			os.Exit(3)
		}()
	}
	return stop
}

func main() {
	defer startProfile()()
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
  goonix eval [--json] [--pure] [-I path]... [-A attr] [--arg name expr] [--argstr name str] <file.nix|dir>
  goonix instantiate [--store DIR] [-I path]... [-A attr] [--arg name expr] [--argstr name str] <file.nix|dir>
  goonix build [--store DIR] [-I path]... [-A attr] [--arg name expr] [--argstr name str] <file.nix|dir>
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
	extraXP := fs.String("extra-experimental-features", "", "space-separated experimental features (flakes enables fetch-tree)")
	ra, flags, err := splitRootArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	_ = fs.Parse(flags)
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "eval expects one file argument")
		os.Exit(2)
	}
	ev := eval.NewEvaluator()
	if *pure {
		ev = ev.WithEffects(eval.ReadOnlyEffects{Effects: ev.Effects})
	} else {
		ev = ev.WithEffects(eval.IOEffects{})
	}
	ev = ev.WithExperimental(strings.Fields(*extraXP)...)
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
	storeFlag := fs.String("store", "", "physical store root (contains store/ subdirectory)")
	ra, flags, err := splitRootArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	_ = fs.Parse(flags)
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(os.Stderr, "instantiate expects one file argument")
		os.Exit(2)
	}
	// --store is the same physical root build uses. Logical paths stay /nix/store.
	var eff eval.Effects = eval.IOEffects{}
	if *storeFlag != "" {
		eff = eval.ChrootIOEffects{PhysRoot: *storeFlag}
	}
	ev := eval.NewEvaluator().WithEffects(eff)
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
	if fv.Kind != eval.VKAttrs {
		fmt.Fprintln(os.Stderr, "error: expression does not evaluate to a derivation (or a set or list of those)")
		os.Exit(1)
	}
	// nix-instantiate prints drvPath!output when the default output is not out.
	suffix := ""
	if out, ok, err := ev.AttrString(fv, "outputName"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	} else if ok && out != "out" {
		suffix = "!" + out
	}
	cfg := store.Config{}
	if drvPath, err := eval.ForceStrictDrvPath(ev, fv); err == nil {
		drv, ok := ev.LookupStrictDerivation(drvPath)
		if !ok {
			fmt.Fprintln(os.Stderr, "derivation was not registered during eval")
			os.Exit(1)
		}
		if err := ev.WriteDerivationClosure(cfg, drv); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(strings.TrimSpace(drv.DrvPath) + suffix)
		return
	}
	drv, err := ev.AttrsToDerivation(fv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := ev.WriteDerivationClosure(cfg, drv); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(strings.TrimSpace(drv.DrvPath) + suffix)
}
