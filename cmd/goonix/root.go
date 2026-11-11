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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// rootArgs holds the nix-instantiate style arguments shared by eval and
// instantiate: -I, -A, --arg and --argstr.
type rootArgs struct {
	search []string
	attr   string
	args   []string // "name = expr;" bindings in Nix syntax
}

// splitRootArgs removes -I, -A, --arg NAME EXPR and --argstr NAME VALUE from
// args and returns the remaining arguments for the flag package. --arg takes
// two values, which the flag package cannot express.
func splitRootArgs(args []string) (rootArgs, []string, error) {
	var ra rootArgs
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		need := func(n int) error {
			if i+n >= len(args) {
				return fmt.Errorf("%s needs %d argument(s)", a, n)
			}
			return nil
		}
		switch {
		case a == "-I" || a == "--include":
			if err := need(1); err != nil {
				return ra, nil, err
			}
			ra.search = append(ra.search, args[i+1])
			i++
		case strings.HasPrefix(a, "-I") && len(a) > 2:
			ra.search = append(ra.search, a[2:])
		case a == "-A" || a == "--attr":
			if err := need(1); err != nil {
				return ra, nil, err
			}
			ra.attr = args[i+1]
			i++
		case a == "--arg" || a == "--argstr":
			if err := need(2); err != nil {
				return ra, nil, err
			}
			val := args[i+2]
			if a == "--argstr" {
				val = nixString(val)
			} else {
				val = "(" + val + ")"
			}
			ra.args = append(ra.args, nixString(args[i+1])+" = "+val+";")
			i += 2
		default:
			rest = append(rest, a)
		}
	}
	return ra, rest, nil
}

// apply adds the -I entries to ev's search path.
func (ra rootArgs) apply(ev *eval.Evaluator) {
	for _, s := range ra.search {
		for _, part := range strings.Split(s, ":") {
			if part = strings.TrimSpace(part); part != "" {
				ev.NixPath.PrependEntries(part)
			}
		}
	}
}

// expr builds the root expression the way nix-instantiate does: import file
// (a directory means its default.nix), auto-call a function result with the
// --arg/--argstr bindings its formals accept, then select the -A path.
func (ra rootArgs) expr(file string) (parser.Expr, string, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "let f = import %s; args = { %s }; r = if builtins.isFunction f then f (builtins.intersectAttrs (builtins.functionArgs f) args) else f; in r",
		nixString(abs), strings.Join(ra.args, " "))
	if ra.attr != "" {
		for _, p := range strings.Split(ra.attr, ".") {
			b.WriteString("." + nixString(p))
		}
	}
	e, err := parser.ParseString(b.String())
	if err != nil {
		return nil, "", err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	return e, filepath.Join(cwd, "(command line)"), nil
}

// nixString quotes s as a Nix double-quoted string literal.
func nixString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "${", `\${`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	return `"` + r.Replace(s) + `"`
}
