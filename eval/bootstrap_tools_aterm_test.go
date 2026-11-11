package eval_test

import (
	"testing"
)

func TestBootstrapToolsATermMatchesNix(t *testing.T) {
	bootstrapATermOracle(t, "bootstrap-tools.drv")
}
