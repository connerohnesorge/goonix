package eval_test

import "testing"

func TestBootstrapStage2XgccATermMatchesNix(t *testing.T) {
	nixKey, goKey, ev, _ := bootstrapStage2XgccPair(t)
	bootstrapATermOracleKeys(t, nixKey, goKey, ev)
}

func TestBootstrapStage2BinutilsWrapperATermMatchesNix(t *testing.T) {
	bootstrapATermOracleLogical(t, "-binutils-wrapper-2.43.1.drv")
}
