// Package dcs provides normalisation and detection for DCS JSON-LD documents.
//
// Document types:
//
//	dcs:ContractTemplate — template with ordered dcs:sections; constraints via odrl:policy
//	dcs:Contract         — contract with resolved parties and odrl:Agreement policies
//
// Detection is by @type field. Normalisation enforces @context, validates the
// minimal required structure, and merges relational metadata (@id, dcs:version, …).
package dcs

import (
	"encoding/json"
	"fmt"
	"strings"

	"digital-contracting-service/internal/base/datatype"
)

// dcsContext is enforced on all DCS JSON-LD documents.
var dcsContext = map[string]any{
	"dcs":     "https://w3id.org/facis/dcs#",
	"odrl":    "http://www.w3.org/ns/odrl/2/",
	"sla":     "https://w3id.org/facis/sla/ontology#",
	"xsd":     "http://www.w3.org/2001/XMLSchema#",
	"dcterms": "http://purl.org/dc/terms/",
}

// ── Detection ─────────────────────────────────────────────────────────────────

// IsTemplate returns true when raw is a dcs:ContractTemplate.
func IsTemplate(raw *datatype.JSON) bool {
	return jsonLdType(raw) == "dcs:ContractTemplate"
}

// IsContract returns true when raw is a dcs:Contract.
func IsContract(raw *datatype.JSON) bool {
	return jsonLdType(raw) == "dcs:Contract"
}

// ── Normalisation ─────────────────────────────────────────────────────────────

// NormalizeTemplate validates and normalises a dcs:ContractTemplate.
// If id is non-empty it is set as @id.
func NormalizeTemplate(raw *datatype.JSON, id string) (*datatype.JSON, error) {
	doc, err := decode(raw)
	if err != nil {
		return nil, fmt.Errorf("dcs template: %w", err)
	}
	doc["@context"] = dcsContext
	doc["@type"] = "dcs:ContractTemplate"
	if id != "" {
		doc["@id"] = id
	}
	if err := validateTemplate(doc); err != nil {
		return nil, err
	}
	return encode(doc)
}

// NormalizeContract validates and normalises a dcs:Contract.
// If id is non-empty it is set as @id.
func NormalizeContract(raw *datatype.JSON, id string) (*datatype.JSON, error) {
	doc, err := decode(raw)
	if err != nil {
		return nil, fmt.Errorf("dcs contract: %w", err)
	}
	doc["@context"] = dcsContext
	doc["@type"] = "dcs:Contract"
	if id != "" {
		doc["@id"] = id
	}
	if err := validateContract(doc); err != nil {
		return nil, err
	}
	return encode(doc)
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateTemplate(doc map[string]any) error {
	if id, _ := doc["@id"].(string); strings.TrimSpace(id) == "" {
		return fmt.Errorf("dcs:ContractTemplate: @id is required")
	}
	sections, err := requireArray(doc, "dcs:sections", "dcs:ContractTemplate", 0)
	if err != nil {
		return err
	}
	for i, item := range sections {
		if err := validateSection(item, i); err != nil {
			return err
		}
	}
	return nil
}

func validateContract(doc map[string]any) error {
	if id, _ := doc["@id"].(string); strings.TrimSpace(id) == "" {
		return fmt.Errorf("dcs:Contract: @id is required")
	}
	if src, _ := doc["dcs:template_source"].(string); strings.TrimSpace(src) == "" {
		return fmt.Errorf("dcs:Contract: dcs:template_source is required")
	}
	sections, err := requireArray(doc, "dcs:sections", "dcs:Contract", 0)
	if err != nil {
		return err
	}
	for i, item := range sections {
		if err := validateSection(item, i); err != nil {
			return err
		}
	}
	return nil
}

func validateSection(raw any, idx int) error {
	section, ok := raw.(map[string]any)
	if !ok {
		return fmt.Errorf("dcs:sections[%d] must be an object", idx)
	}
	t, _ := section["@type"].(string)
	switch t {
	case "dcs:TextBlock":
		if _, ok := section["dcs:content"]; !ok {
			return fmt.Errorf("dcs:sections[%d] (dcs:TextBlock) dcs:content is required", idx)
		}
	case "dcs:Clause":
		if _, ok := section["dcs:content"]; !ok {
			return fmt.Errorf("dcs:sections[%d] (dcs:Clause) dcs:content is required", idx)
		}
		if err := validateOdrlPolicy(section, idx); err != nil {
			return err
		}
	default:
		return fmt.Errorf("dcs:sections[%d] @type must be dcs:TextBlock or dcs:Clause, got %q", idx, t)
	}
	return nil
}

func validateOdrlPolicy(section map[string]any, idx int) error {
	rawPolicy, exists := section["odrl:policy"]
	if !exists {
		// A clause without an odrl:policy is valid (prose-only clause).
		return nil
	}
	policy, ok := rawPolicy.(map[string]any)
	if !ok {
		return fmt.Errorf("dcs:sections[%d] odrl:policy must be an object", idx)
	}
	t, _ := policy["@type"].(string)
	if t != "odrl:Set" && t != "odrl:Agreement" {
		return fmt.Errorf("dcs:sections[%d] odrl:policy @type must be odrl:Set or odrl:Agreement, got %q", idx, t)
	}
	for _, ruleKey := range []string{"odrl:permission", "odrl:prohibition", "odrl:obligation"} {
		rawRules, ok := policy[ruleKey]
		if !ok {
			continue
		}
		rules, ok := rawRules.([]any)
		if !ok {
			return fmt.Errorf("dcs:sections[%d] odrl:policy %s must be an array", idx, ruleKey)
		}
		for ri, rawRule := range rules {
			rule, ok := rawRule.(map[string]any)
			if !ok {
				return fmt.Errorf("dcs:sections[%d] odrl:policy %s[%d] must be an object", idx, ruleKey, ri)
			}
			if _, ok := rule["odrl:action"]; !ok {
				return fmt.Errorf("dcs:sections[%d] odrl:policy %s[%d] odrl:action is required", idx, ruleKey, ri)
			}
			if err := validateConstraints(rule, idx, ruleKey, ri); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateConstraints(rule map[string]any, sectionIdx int, ruleKey string, ruleIdx int) error {
	rawConstraints, ok := rule["odrl:constraint"]
	if !ok {
		return nil
	}
	constraints, ok := rawConstraints.([]any)
	if !ok {
		return fmt.Errorf("dcs:sections[%d] %s[%d] odrl:constraint must be an array", sectionIdx, ruleKey, ruleIdx)
	}
	for ci, rawC := range constraints {
		c, ok := rawC.(map[string]any)
		if !ok {
			return fmt.Errorf("dcs:sections[%d] %s[%d] odrl:constraint[%d] must be an object", sectionIdx, ruleKey, ruleIdx, ci)
		}
		if _, ok := c["odrl:leftOperand"]; !ok {
			return fmt.Errorf("dcs:sections[%d] %s[%d] odrl:constraint[%d] odrl:leftOperand is required", sectionIdx, ruleKey, ruleIdx, ci)
		}
		if _, ok := c["odrl:operator"]; !ok {
			return fmt.Errorf("dcs:sections[%d] %s[%d] odrl:constraint[%d] odrl:operator is required", sectionIdx, ruleKey, ruleIdx, ci)
		}
		if _, ok := c["odrl:rightOperand"]; !ok {
			return fmt.Errorf("dcs:sections[%d] %s[%d] odrl:constraint[%d] odrl:rightOperand is required", sectionIdx, ruleKey, ruleIdx, ci)
		}
	}
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func jsonLdType(raw *datatype.JSON) string {
	if raw == nil || !raw.IsNotNullValue() {
		return ""
	}
	var peek struct {
		Type string `json:"@type"`
	}
	_ = json.Unmarshal(*raw, &peek)
	return peek.Type
}

func decode(raw *datatype.JSON) (map[string]any, error) {
	if raw == nil || !raw.IsNotNullValue() {
		return nil, fmt.Errorf("document is empty")
	}
	var doc map[string]any
	if err := json.Unmarshal(*raw, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func encode(doc map[string]any) (*datatype.JSON, error) {
	j, err := datatype.NewJSON(doc)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

func requireArray(doc map[string]any, key, typeName string, min int) ([]any, error) {
	raw, exists := doc[key]
	if !exists {
		if min > 0 {
			return nil, fmt.Errorf("%s: %q is required", typeName, key)
		}
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: %q must be an array", typeName, key)
	}
	if len(arr) < min {
		return nil, fmt.Errorf("%s: %q must have at least %d element(s)", typeName, key, min)
	}
	return arr, nil
}
