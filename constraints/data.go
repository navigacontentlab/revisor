package constraints

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/ttab/revisor"
)

//go:embed core.json
var coreSchema []byte

func CoreSchemaVersion() string {
	return "v1.0.1"
}

func CoreSchema() (revisor.ConstraintSet, error) {
	dec := json.NewDecoder(bytes.NewReader(coreSchema))

	dec.DisallowUnknownFields()

	var spec revisor.ConstraintSet

	err := dec.Decode(&spec)
	if err != nil {
		return revisor.ConstraintSet{}, fmt.Errorf(
			"failed to unmarshal core constraints: %w", err)
	}

	return spec, nil
}
