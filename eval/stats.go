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

package eval

import (
	"fmt"
	"io"
	"os"
	"sort"
	"sync/atomic"

	"github.com/connerohnesorge/goonix/parser"
)

// GOONIX_EVAL_STATS=1 counts thunk evaluations and lambda calls per source
// position. RequestStatsDump asks the evaluating goroutine to print the top
// entries to stderr and exit, which is how a run that never finishes shows
// where its time goes.
var (
	statsOn      = os.Getenv("GOONIX_EVAL_STATS") != ""
	statsDumpReq atomic.Bool
	thunkCounts  = map[parser.Pos]int{}
	lambdaCounts = map[parser.Pos]int{}
)

// RequestStatsDump makes the evaluator dump GOONIX_EVAL_STATS counters and
// exit with status 3 at its next thunk evaluation.
func RequestStatsDump() { statsDumpReq.Store(true) }

func countThunk(pos parser.Pos) {
	thunkCounts[pos]++
	if statsDumpReq.Load() {
		DumpStats(os.Stderr)
		os.Exit(3)
	}
}

func countLambda(pos parser.Pos) { lambdaCounts[pos]++ }

// DumpStats prints the 40 most frequent thunk and lambda positions.
func DumpStats(w io.Writer) {
	for _, s := range []struct {
		name string
		m    map[parser.Pos]int
	}{{"thunk evaluations", thunkCounts}, {"lambda calls", lambdaCounts}} {
		type kv struct {
			p parser.Pos
			n int
		}
		var all []kv
		total := 0
		for p, n := range s.m {
			all = append(all, kv{p, n})
			total += n
		}
		sort.Slice(all, func(i, j int) bool { return all[i].n > all[j].n })
		fmt.Fprintf(w, "== %s: %d total\n", s.name, total)
		for i := 0; i < len(all) && i < 40; i++ {
			fmt.Fprintf(w, "%10d  %s:%d:%d\n", all[i].n, all[i].p.File, all[i].p.Line, all[i].p.Col)
		}
	}
}
