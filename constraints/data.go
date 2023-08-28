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

//go:embed core_planning.json
var corePlanningSchema []byte

type schemaData struct {
	Name string
	Data []byte
}

var specifications = []schemaData{
	{Name: "core", Data: coreSchema},
	{Name: "core-planning", Data: corePlanningSchema},
}

func CoreSchemaVersion() string {
	return "v1.1.0"
}

func CoreSchema() ([]revisor.ConstraintSet, error) {
	var core []revisor.ConstraintSet

	for _, s := range specifications {
		var cs revisor.ConstraintSet

		err := strictUnmarshalBytes(s.Name, s.Data, &cs)
		if err != nil {
			return nil, err
		}

		core = append(core, cs)
	}

	return core, nil
}

func strictUnmarshalBytes(name string, data []byte, o any) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	dec.DisallowUnknownFields()

	err := dec.Decode(o)
	if err != nil {
		return fmt.Errorf(
			"failed to unmarshal %s: %w",
			name, err)
	}

	return nil
}
