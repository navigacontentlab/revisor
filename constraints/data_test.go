package constraints_test

import (
	"testing"

	"github.com/ttab/revisor/constraints"
)

func TestCore(t *testing.T) {
	spec, err := constraints.CoreSchema()
	if err != nil {
		t.Fatalf("failed to load core schema: %v", err)
	}

	if len(spec) == 0 {
		t.Fatalf("expected one or more core constraint sets")
	}
}
