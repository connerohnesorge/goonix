package eval_test

import "testing"

func TestBootstrapStage2XgccATermMatchesNix(t *testing.T) {
	bootstrapATermOracleLogical(t, "-xgcc-13.3.0.drv")
}

func TestBootstrapStage2BinutilsWrapperATermMatchesNix(t *testing.T) {
	bootstrapATermOracleLogical(t, "-binutils-wrapper-2.43.1.drv")
}
