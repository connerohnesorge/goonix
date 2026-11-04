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
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func cmdDerivationShow(args []string) {
	fs := flag.NewFlagSet("derivation-show", flag.ExitOnError)
	goEval := fs.String("go-eval", "", "evaluate FILE and treat result as a .drv path or derivation")
	nixDrv := fs.String("nix-drv", "", "materialised nix .drv path")
	nixInst := fs.String("nix-instantiate", "", "nix-instantiate FILE to get nix top .drv")
	_ = fs.Parse(args)

	lay, err := oracle.FindLayout()
	if err != nil {
		fatal(err)
	}

	if *nixDrv != "" {
		doc, err := drvshow.NixClosure(*nixDrv)
		if err != nil {
			fatal(err)
		}
		emitJSON(doc)
		return
	}
	if *nixInst != "" {
		p, err := drvshow.MaterializeNixDrv(lay, *nixInst)
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
		ev, path, err := evalFileDrvPath(lay, *goEval)
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
	_ = fs.Parse(args)
	if *goEval == "" || (*nixDrv == "" && *nixInst == "") {
		fmt.Fprintln(os.Stderr, "drv-diff requires --go-eval and (--nix-drv or --nix-instantiate)")
		os.Exit(2)
	}
	lay, err := oracle.FindLayout()
	if err != nil {
		fatal(err)
	}
	var nixPath string
	if *nixInst != "" {
		nixPath, err = drvshow.MaterializeNixDrv(lay, *nixInst)
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
	ev, goPath, err := evalFileDrvPath(lay, *goEval)
	if err != nil {
		fatal(err)
	}
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		fatal(err)
	}
	report, err := drvdiff.AnalyseWithATerm(nixDoc, goDoc, drvshow.DrvKey(nixPath), drvshow.DrvKey(goPath), drvdiff.StoreATermSource{
		Go: func(goDrvKey string) []byte {
			gd, ok := ev.RegisteredDerivation("/nix/store/" + goDrvKey)
			if !ok {
				return nil
			}
			b, _ := eval.SerializeGoNixDerivation(gd)
			return b
		},
	})
	if err != nil {
		fatal(err)
	}
	fmt.Print(drvdiff.FormatReport(report))
	if report.ByteFirst != nil {
		fmt.Print(drvdiff.FormatByteCulprit(report.ByteFirst))
		return
	}
	if report.Deepest != nil {
		nixEntry := nixDoc.Derivations[report.Deepest.NixKey]
		goEntry := goDoc.Derivations[report.Deepest.GoonixKey]
		nixATerm := drvdiff.ReadNixStoreATerm(report.Deepest.NixKey)
		goFull := "/nix/store/" + report.Deepest.GoonixKey
		var goATerm []byte
		if gd, ok := ev.RegisteredDerivation(goFull); ok {
			goATerm, _ = eval.SerializeGoNixDerivation(gd)
		}
		fmt.Print(drvdiff.FormatMismatchDetail(report.Deepest, nixEntry, goEntry, nixATerm, goATerm))
	}
}

func evalFileDrvPath(lay oracle.Layout, path string) (*eval.Evaluator, string, error) {
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(path)
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
		return ev, strings.TrimSpace(fv.String), nil
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
