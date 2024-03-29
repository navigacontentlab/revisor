package revisor

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/navigacontentlab/navigadoc/doc"
)

type Validator struct {
	constraints []ConstraintSet

	documents            []*DocumentConstraint
	properties           []PropertyConstraintMap
	blockConstraints     []BlockConstraintSet
	attributeConstraints []ConstraintMap
	htmlPolicies         map[string]*HTMLPolicy
}

func NewValidator(
	constraints ...ConstraintSet,
) (*Validator, error) {
	v := Validator{
		constraints:  constraints,
		htmlPolicies: make(map[string]*HTMLPolicy),
	}

	docDeclared := make(map[string]bool)

	policySet := NewHTMLPolicySet()

	for _, constraint := range constraints {
		err := constraint.Validate()
		if err != nil {
			return nil, fmt.Errorf("constraint set %q is not valid: %w",
				constraint.Name, err)
		}

		v.properties = append(v.properties, constraint.Properties)
		v.blockConstraints = append(v.blockConstraints, constraint)
		v.attributeConstraints = append(v.attributeConstraints, constraint.Attributes)

		for j := range constraint.Documents {
			d := constraint.Documents[j]

			v.documents = append(v.documents, &d)

			if d.Declares == "" {
				continue
			}

			if docDeclared[d.Declares] {
				return nil, fmt.Errorf("document type %q redeclared in %q",
					d.Declares, constraint.Name)
			}

			docDeclared[d.Declares] = true
		}

		err = policySet.Add(constraint.Name, constraint.HTMLPolicies...)
		if err != nil {
			return nil, fmt.Errorf("failed to add HTML policies for %q: %w",
				constraint.Name, err)
		}
	}

	htmlPolicies, err := policySet.Resolve()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve HTML policies: %w", err)
	}

	v.htmlPolicies = htmlPolicies

	return &v, nil
}

type ValidationResult struct {
	Entity []EntityRef `json:"entity,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func (vr ValidationResult) String() string {
	if len(vr.Entity) > 0 {
		return entityRefsToString(vr.Entity) + ": " + vr.Error
	}

	return vr.Error
}

func entityRefsToString(refs []EntityRef) string {
	l := len(refs)
	r := make([]string, l)

	for i, v := range refs {
		r[i] = v.String()
	}

	return strings.Join(r, " of ")
}

type RefType string

const (
	RefTypeBlock     RefType = "block"
	RefTypeProperty  RefType = "property"
	RefTypeAttribute RefType = "attribute"
	RefTypeData      RefType = "data attribute"
	RefTypeParameter RefType = "parameter"
)

func (rt RefType) String() string {
	return string(rt)
}

type EntityRef struct {
	RefType   RefType   `json:"refType"`
	BlockKind BlockKind `json:"kind,omitempty"`
	Index     int       `json:"index,omitempty"`
	Name      string    `json:"name,omitempty"`
	Type      string    `json:"type,omitempty"`
	Rel       string    `json:"rel,omitempty"`
}

func (er EntityRef) String() string {
	if er.RefType == RefTypeBlock {
		return fmt.Sprintf("%s %d %s",
			er.BlockKind.Description(1),
			er.Index+1,
			er.typeDesc(),
		)
	}

	return fmt.Sprintf("%s %q", er.RefType.String(), er.Name)
}

func (er EntityRef) typeDesc() string {
	if er.Type == "" && er.Rel == "" {
		return ""
	}

	if er.Type != "" && er.Rel != "" {
		return fmt.Sprintf("%s(%s)", er.Rel, er.Type)
	}

	if er.Type != "" {
		return fmt.Sprintf("(%s)", er.Type)
	}

	return er.Rel
}

func (v *Validator) validateHTML(policyName string, value string) error {
	if policyName == "" {
		policyName = "default"
	}

	policy, ok := v.htmlPolicies[policyName]
	if !ok {
		return fmt.Errorf("no %q HTML policy defined", policyName)
	}

	return policy.Check(value)
}

func (v *Validator) ValidateDocument(document *doc.Document) []ValidationResult {
	var res []ValidationResult

	propertyConstraints := append([]PropertyConstraintMap{}, v.properties...)
	blockConstraints := append([]BlockConstraintSet{}, v.blockConstraints...)
	attributeConstraints := append([]ConstraintMap{}, v.attributeConstraints...)

	var declared bool

	vCtx := ValidationContext{
		ValidateHTML: v.validateHTML,
	}

	for i := range v.documents {
		match := v.documents[i].Matches(document, &vCtx)
		if match == NoMatch {
			continue
		}

		if match == MatchDeclaration {
			declared = true
		}

		res = v.documents[i].checkAttributes(document, res, &vCtx)

		blockConstraints = append(blockConstraints, v.documents[i])
		propertyConstraints = append(propertyConstraints, v.documents[i].Properties)
		attributeConstraints = append(attributeConstraints, v.documents[i].Attributes)
	}

	if !declared {
		res = append(res, ValidationResult{
			Error: fmt.Sprintf("undeclared document type %q", document.Type),
		})
	}

	res = v.validateBlocks(
		NewDocumentBlocks(document), nil,
		blockConstraints, res,
	)

	res = validateDocumentProperties(document, propertyConstraints, res, &vCtx)

	res = validateDocumentAttributes(attributeConstraints, document, res, vCtx)

	return res
}

func validateDocumentAttributes(
	constraints []ConstraintMap, d *doc.Document,
	res []ValidationResult, vCtx ValidationContext,
) []ValidationResult {
	vCtx.TemplateData = TemplateValues{
		"this": DocumentTemplateValue(d),
	}

	for i := range constraints {
		for k, check := range constraints[i] {
			value, ok := documentAttribute(d, k)

			err := check.Validate(value, ok, &vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{{
						RefType: RefTypeAttribute,
						Name:    k,
					}},
					Error: err.Error(),
				})
			}
		}
	}

	return res
}

func (v *Validator) validateBlocks(
	blocks BlockSource, parent *doc.Block,
	constraints []BlockConstraintSet, res []ValidationResult,
) []ValidationResult {
	vCtx := ValidationContext{
		ValidateHTML: v.validateHTML,
		TemplateData: TemplateValues{
			"parent": BlockTemplateValue(parent),
		},
	}

	for i := range blockKinds {
		res = v.validateBlockSlice(
			blocks.GetBlocks(blockKinds[i]), vCtx,
			constraints, blockKinds[i],
			res,
		)
	}

	return res
}

func (v *Validator) validateBlockSlice(
	blocks []doc.Block, vCtx ValidationContext,
	constraints []BlockConstraintSet, kind BlockKind,
	res []ValidationResult,
) []ValidationResult {
	matches := make(map[*BlockConstraint]int)

	for i := range blocks {
		vCtx.TemplateData["this"] = BlockTemplateValue(&blocks[i])

		r := v.validateBlock(&blocks[i], vCtx, constraints, kind, matches, nil)
		for j := range r {
			r[j].Entity = append(r[j].Entity, EntityRef{
				RefType:   RefTypeBlock,
				Index:     i,
				BlockKind: kind,
				Type:      blocks[i].Type,
				Rel:       blocks[i].Rel,
			})
		}

		res = append(res, r...)
	}

	for i := range constraints {
		for _, constraint := range constraints[i].BlockConstraints(kind) {
			count := matches[constraint]

			valid := nilOrEqual(constraint.Count, count) &&
				nilOrGTE(constraint.MinCount, count) &&
				nilOrLTE(constraint.MaxCount, count)
			if !valid {
				res = append(res, ValidationResult{
					Error: constraint.DescribeCountConstraint(kind),
				})
			}
		}
	}

	return res
}

func nilOrEqual(t *int, n int) bool {
	if t == nil {
		return true
	}

	return *t == n
}

func nilOrLTE(t *int, n int) bool {
	if t == nil {
		return true
	}

	return n <= *t
}

func nilOrGTE(t *int, n int) bool {
	if t == nil {
		return true
	}

	return n >= *t
}

func (v *Validator) validateBlock(
	b *doc.Block, vCtx ValidationContext,
	constraintSets []BlockConstraintSet, kind BlockKind,
	matches map[*BlockConstraint]int, res []ValidationResult,
) []ValidationResult {
	var (
		defined                     bool
		matchedConstraints          []BlockConstraintSet
		matchedDataConstraints      []ConstraintMap
		matchedAttributeConstraints []ConstraintMap
	)

	declaredAttributes := make(map[blockAttributeKey]bool)

	for _, set := range constraintSets {
		constraints := set.BlockConstraints(kind)

		for _, constraint := range constraints {
			match, attributes := constraint.Matches(b)
			if match == NoMatch {
				continue
			}

			if match == MatchDeclaration {
				defined = true
			}

			for i := range attributes {
				declaredAttributes[blockAttributeKey(attributes[i])] = true
			}

			matches[constraint]++

			matchedConstraints = append(
				matchedConstraints, constraint)

			if len(constraint.BlocksFrom) > 0 {
				matchedConstraints = append(
					matchedConstraints,
					v.borrowedBlockConstraints(constraint.BlocksFrom, vCtx)...,
				)
			}

			matchedDataConstraints = append(
				matchedDataConstraints, constraint.Data)

			matchedAttributeConstraints = append(
				matchedAttributeConstraints, constraint.Attributes)
		}
	}

	if !defined {
		res = append(res, ValidationResult{
			Error: "undeclared block type or rel",
		})
	}

	res = validateBlockAttributes(
		declaredAttributes,
		matchedAttributeConstraints, b, vCtx, res)
	res = validateBlockData(b.Data, vCtx, matchedDataConstraints, res)

	res = v.validateBlocks(
		NewNestedBlocks(b), b,
		matchedConstraints, res,
	)

	return res
}

func (v *Validator) borrowedBlockConstraints(
	list []BlocksFrom, vCtx ValidationContext,
) []BlockConstraintSet {
	var match []BlockConstraintSet

	for _, borrow := range list {
		if borrow.Global {
			for _, c := range v.blockConstraints {
				match = append(
					match,
					BorrowedBlocks{
						Kind:   borrow.Kind,
						Source: c,
					},
				)
			}
		}

		if borrow.DocType != "" {
			dummyArt := doc.Document{
				Type: borrow.DocType,
			}

			for _, d := range v.documents {
				if d.Matches(&dummyArt, &vCtx) == NoMatch {
					continue
				}

				match = append(
					match,
					BorrowedBlocks{
						Kind:   borrow.Kind,
						Source: d,
					},
				)
			}
		}
	}

	return match
}

func validateBlockAttributes(
	declaredAttributes map[blockAttributeKey]bool,
	constraints []ConstraintMap, b *doc.Block, vCtx ValidationContext,
	res []ValidationResult,
) []ValidationResult {
	if b.UUID != "" {
		_, err := uuid.Parse(b.UUID)
		if err != nil {
			res = append(res, ValidationResult{
				Entity: []EntityRef{{
					RefType: RefTypeAttribute,
					Name:    string(blockAttrUUID),
				}},
				Error: err.Error(),
			})
		}
	}

	for i := range constraints {
		for k, check := range constraints[i] {
			value, ok := blockAttribute(b, k)

			err := check.Validate(value, ok, &vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{{
						RefType: RefTypeAttribute,
						Name:    k,
					}},
					Error: err.Error(),
				})
			}

			declaredAttributes[blockAttributeKey(k)] = true
		}
	}

	for i := range allBlockAttributes {
		if declaredAttributes[allBlockAttributes[i]] {
			continue
		}

		k := string(allBlockAttributes[i])

		value, ok := blockAttribute(b, k)
		if ok && value != "" {
			res = append(res, ValidationResult{
				Entity: []EntityRef{{
					RefType: RefTypeAttribute,
					Name:    k,
				}},
				Error: "undeclared block attribute",
			})
		}
	}

	return res
}

func validateBlockData(
	data map[string]string, vCtx ValidationContext, constraints []ConstraintMap,
	res []ValidationResult,
) []ValidationResult {
	known := make(map[string]bool)

	for i := range constraints {
		for k, check := range constraints[i] {
			var (
				v  string
				ok bool
			)

			if data != nil {
				v, ok = data[k]
			}

			known[k] = known[k] || ok

			if !ok && !check.Optional {
				res = append(res, ValidationResult{
					Entity: []EntityRef{{
						RefType: RefTypeData,
						Name:    k,
					}},
					Error: "missing required attribute",
				})
			}

			if !ok {
				continue
			}

			err := check.Validate(v, true, &vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{{
						RefType: RefTypeData,
						Name:    k,
					}},
					Error: err.Error(),
				})
			}
		}
	}

	for k := range data {
		if known[k] {
			continue
		}

		res = append(res, ValidationResult{
			Entity: []EntityRef{{
				RefType: RefTypeData,
				Name:    k,
			}},
			Error: "unknown attribute",
		})
	}

	return res
}

type BlockConstraintSet interface {
	// BlockConstraints returns the constraints of the specified kind.
	BlockConstraints(kind BlockKind) []*BlockConstraint
}

type ConstraintSet struct {
	Version      int                   `json:"version,omitempty"`
	Schema       string                `json:"$schema,omitempty"`
	Name         string                `json:"name"`
	Documents    []DocumentConstraint  `json:"documents,omitempty"`
	Links        []*BlockConstraint    `json:"links,omitempty"`
	Meta         []*BlockConstraint    `json:"meta,omitempty"`
	Content      []*BlockConstraint    `json:"content,omitempty"`
	Properties   PropertyConstraintMap `json:"properties,omitempty"`
	Attributes   ConstraintMap         `json:"attributes,omitempty"`
	HTMLPolicies []HTMLPolicy          `json:"htmlPolicies,omitempty"`
}

func (cs ConstraintSet) Validate() error {
	err := validateBlockConstraints(map[string][]*BlockConstraint{
		"link":    cs.Links,
		"meta":    cs.Meta,
		"content": cs.Content,
	})
	if err != nil {
		return err
	}

	for i, d := range cs.Documents {
		err := validateBlockConstraints(map[string][]*BlockConstraint{
			"link":    d.Links,
			"meta":    d.Meta,
			"content": d.Content,
		})
		if err != nil {
			return fmt.Errorf("document %d: %w", i+1, err)
		}
	}

	return nil
}

func validateBlockConstraints(c map[string][]*BlockConstraint) error {
	for k := range c {
		for i, block := range c[k] {
			if block == nil {
				return fmt.Errorf("%s block %d must not be nil/null", k, i+1)
			}

			err := validateBlockConstraints(map[string][]*BlockConstraint{
				"link":    block.Links,
				"meta":    block.Meta,
				"content": block.Content,
			})
			if err != nil {
				return fmt.Errorf("%s block %d: %w", k, i+1, err)
			}
		}
	}

	return nil
}

func (cs ConstraintSet) BlockConstraints(kind BlockKind) []*BlockConstraint {
	switch kind {
	case BlockKindLink:
		return cs.Links
	case BlockKindMeta:
		return cs.Meta
	case BlockKindContent:
		return cs.Content
	}

	return nil
}
