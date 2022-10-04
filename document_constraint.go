package revisor

import (
	"fmt"

	"github.com/navigacontentlab/navigadoc/doc"
)

// DocumentConstraint describes a set of constraints for a document. Either by
// declaring a document type, or matching against a document that has been
// declared somewhere else.
type DocumentConstraint struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// Declares is used to declare a document type.
	Declares string `json:"declares,omitempty"`
	// Match is used to extend other document declarations.
	Match      ConstraintMap         `json:"match,omitempty"`
	Links      []*BlockConstraint    `json:"links,omitempty"`
	Meta       []*BlockConstraint    `json:"meta,omitempty"`
	Content    []*BlockConstraint    `json:"content,omitempty"`
	Properties PropertyConstraintMap `json:"properties,omitempty"`
	Attributes ConstraintMap         `json:"attributes,omitempty"`
}

// BlockConstraints implements the BlockConstraintsSet interface.
func (dc DocumentConstraint) BlockConstraints(kind BlockKind) []*BlockConstraint {
	switch kind {
	case BlockKindLink:
		return dc.Links
	case BlockKindMeta:
		return dc.Meta
	case BlockKindContent:
		return dc.Content
	}

	return nil
}

// Matches checks if the given document matches the constraint.
func (dc DocumentConstraint) Matches(
	d *doc.Document, vCtx *ValidationContext,
) Match {
	if dc.Declares != "" {
		if d.Type == dc.Declares {
			return MatchDeclaration
		}

		return NoMatch
	}

	for k, check := range dc.Match {
		value, ok := documentMatchAttribute(d, k)
		if !ok {
			return NoMatch
		}

		err := check.Validate(value, ok, vCtx)
		if err != nil {
			return NoMatch
		}
	}

	return Matches
}

func (dc DocumentConstraint) checkAttributes(
	d *doc.Document, res []ValidationResult, vCtx *ValidationContext,
) []ValidationResult {
	vCtx.TemplateData = TemplateValues{
		"this": DocumentTemplateValue(d),
	}

	for k, check := range dc.Attributes {
		value, ok := documentAttribute(d, k)
		if !ok {
			res = append(res, ValidationResult{
				Error: fmt.Sprintf("unknown document attribute %q", k),
			})

			continue
		}

		err := check.Validate(value, ok, vCtx)
		if err != nil {
			res = append(res, ValidationResult{
				Error: fmt.Sprintf("invalid %q: %v", k, err),
			})

			continue
		}
	}

	return res
}

func validateDocumentProperties(
	d *doc.Document, constraints []PropertyConstraintMap,
	res []ValidationResult, vCtx *ValidationContext,
) []ValidationResult {
	counts := make(map[string]int)

	vCtx.TemplateData = TemplateValues{
		"parent": DocumentTemplateValue(d),
	}

	for i := range d.Properties {
		var matched bool

		vCtx.TemplateData["this"] = PropertyTemplateValue(&d.Properties[i])

		res, matched = checkProperty(
			d.Properties[i], constraints, vCtx, res,
		)

		if matched {
			counts[d.Properties[i].Name]++
		}
	}

	for i := range constraints {
		for name, constraint := range constraints[i] {
			count := counts[name]

			valid := nilOrEqual(constraint.Count, count) &&
				nilOrGTE(constraint.MinCount, count) &&
				nilOrLTE(constraint.MaxCount, count)
			if !valid {
				res = append(res, ValidationResult{
					Entity: []EntityRef{
						{
							RefType: RefTypeProperty,
							Name:    name,
						},
					},
					Error: constraint.DescribeCountConstraint(name),
				})
			}
		}
	}

	return res
}

type documentAttributeKey string

const (
	docAttrType     documentAttributeKey = "type"
	docAttrLanguage documentAttributeKey = "language"
	docAttrStatus   documentAttributeKey = "status"
	docAttrTitle    documentAttributeKey = "title"
	docAttrProvider documentAttributeKey = "provider"
	docAttrSubtype  documentAttributeKey = "subtype"
	docAttrUUID     documentAttributeKey = "uuid"
	docAttrURI      documentAttributeKey = "uri"
	docAttrURL      documentAttributeKey = "url"
	docAttrPath     documentAttributeKey = "path"
)

func documentMatchAttribute(d *doc.Document, name string) (string, bool) {
	//nolint:exhaustive
	switch documentAttributeKey(name) {
	case docAttrType:
		return d.Type, true
	case docAttrSubtype:
		for i := range d.Properties {
			if d.Properties[i].Name == "subtype" {
				return d.Properties[i].Value, true
			}
		}

		return "", false
	case docAttrProvider:
		return d.Provider, true
	}

	return "", false
}

func documentAttribute(d *doc.Document, name string) (string, bool) {
	switch documentAttributeKey(name) {
	case docAttrUUID:
		return d.UUID, true
	case docAttrType:
		return d.Type, true
	case docAttrURI:
		return d.URI, true
	case docAttrURL:
		return d.URL, true
	case docAttrTitle:
		return d.Title, true
	case docAttrPath:
		return d.Path, true
	case docAttrLanguage:
		return d.Language, true
	case docAttrStatus:
		return d.Status, true
	case docAttrProvider:
		return d.Provider, true
	case docAttrSubtype:
		for i := range d.Properties {
			if d.Properties[i].Name == "subtype" {
				return d.Properties[i].Value, true
			}
		}

		return "", false
	}

	return "", false
}
