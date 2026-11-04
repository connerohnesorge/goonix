// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/drvdiff"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func cmdDerivationShow(args []string) {
	fs := flag.NewFlagSet("derivation-show", flag.ExitOnError)
	goEval := fs.String("go-eval", "", "evaluate FILE and treat result as a .drv path or derivation")
	nixDrv := fs.String("nix-drv", "", "materialised nix .drv path")
	nixInst := fs.String("nix-instantiate", "", "nix-instantiate FILE to get nix top .drv")
	searchPath := fs.String("I", "", "NIXPATH entries (repeatable with comma separation)")
	_ = fs.Parse(args)

	if *nixDrv != "" {
		doc, err := drvshow.NixClosure(*nixDrv)
		if err != nil {
			fatal(err)
		}
		emitJSON(doc)
		return
	}
	if *nixInst != "" {
		env := os.Environ()
		if *searchPath != "" {
			env = append(env, "NIXPATH="+*searchPath)
		}
		p, err := drvshow.MaterializeNixDrv(*nixInst, env)
		if err != nil {
			fatal(err)
		}
		doc, err := drvshow.NixClosure(p)
		if err != nil {
			fatal(err)
		}
		emitJSON(doc)
		return
	}
	if *goEval != "" {
		ev, path, err := evalFileDrvPath(*goEval, *searchPath)
		if err != nil {
			fatal(err)
		}
		doc, err := ev.DerivationShowClosure(path)
		if err != nil {
			fatal(err)
		}
		emitJSON(doc)
		return
	}
	fmt.Fprintln(os.Stderr, "derivation-show: pass --nix-drv, --nix-instantiate, or --go-eval")
	os.Exit(2)
}

func cmdDrvDiff(args []string) {
	fs := flag.NewFlagSet("drv-diff", flag.ExitOnError)
	goEval := fs.String("go-eval", "", "goonix eval file returning .drvPath")
	nixDrv := fs.String("nix-drv", "", "nix top .drv path")
	nixInst := fs.String("nix-instantiate", "", "nix-instantiate derivation file")
	searchPath := fs.String("I", "", "NIXPATH for both sides")
	_ = fs.Parse(args)
	if *goEval == "" || (*nixDrv == "" && *nixInst == "") {
		fmt.Fprintln(os.Stderr, "drv-diff requires --go-eval and (--nix-drv or --nix-instantiate)")
		os.Exit(2)
	}
	env := os.Environ()
	if *searchPath != "" {
		env = append(env, "NIXPATH="+*searchPath)
	}
	var nixPath string
	var err error
	if *nixInst != "" {
		nixPath, err = drvshow.MaterializeNixDrv(*nixInst, env)
	} else {
		nixPath = *nixDrv
	}
	if err != nil {
		fatal(err)
	}
	nixDoc, err := drvshow.NixClosure(nixPath)
	if err != nil {
		fatal(err)
	}
	ev, goPath, err := evalFileDrvPath(*goEval, *searchPath)
	if err != nil {
		fatal(err)
	}
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		fatal(err)
	}
	report, err := drvdiff.Analyse(nixDoc, goDoc, drvshow.DrvKey(nixPath), drvshow.DrvKey(goPath))
	if err != nil {
		fatal(err)
	}
	fmt.Print(drvdiff.FormatReport(report))
}

func evalFileDrvPath(path, nixPath string) (*eval.Evaluator, string, error) {
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.SetCurFile(path)
	if nixPath != "" {
		for _, part := range strings.FieldsFunc(nixPath, func(r rune) bool { return r == ':' || r == ',' }) {
			part = strings.TrimSpace(part)
			if part != "" {
				ev.NixPath.PrependEntries(part)
			}
		}
	}
	expr, err := parser.ParseFile(path)
	if err != nil {
		return nil, "", err
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		return nil, "", err
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		return nil, "", err
	}
	switch fv.Kind {
	case eval.VKPath:
		return ev, fv.Path, nil
	case eval.VKString:
		return ev, fv.String, nil
	case eval.VKAttrs:
		p, err := eval.ForceDerivationAttrs(ev, fv)
		return ev, p, err
	default:
		return nil, "", fmt.Errorf("eval result is not a drv path")
	}
}

func emitJSON(v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(b))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
