package revisor_test

import (
	"fmt"
	"testing"

	"github.com/navigacontentlab/navigadoc/doc"
	"github.com/navigacontentlab/revisor"
	"github.com/navigacontentlab/revisor/internal"
)

func BenchmarkValidateDocument(b *testing.B) {
	var (
		document         doc.Document
		constraints      revisor.ConstraintSet
		extraConstraints revisor.ConstraintSet
	)

	err := internal.UnmarshalFile("constraints/naviga.json", &constraints)
	if err != nil {
		panic(fmt.Errorf(
			"failed to load constraints: %w", err))
	}

	err = internal.UnmarshalFile("constraints/customer.json", &extraConstraints)
	if err != nil {
		panic(fmt.Errorf(
			"failed to load constraints: %w", err))
	}

	err = internal.UnmarshalFile("testdata/example-article.json", &document)
	if err != nil {
		panic(fmt.Errorf(
			"failed to load constraints: %w", err))
	}

	validator, err := revisor.NewValidator(constraints, extraConstraints)
	if err != nil {
		panic(fmt.Errorf("failed to create validator: %w", err))
	}

	for n := 0; n < b.N; n++ {
		_ = validator.ValidateDocument(&document)
	}
}
