// Package mapper builds canonical JSON-LD envelopes from DCS database rows.
//
// dcs:ContractTemplate and dcs:Contract documents (new format) are returned
// as-is with relational fields (DID, version, dates) merged on top.
//
// Legacy DocumentStructure documents fall back to the promotion-based envelope.
package mapper

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"digital-contracting-service/internal/base/datatype"
	"digital-contracting-service/internal/dcs"
	contractdb "digital-contracting-service/internal/contractworkflowengine/db"
	templatedb "digital-contracting-service/internal/templaterepository/db"
)

const (
	jsonLDContextV1     = "https://w3id.org/facis/dcs/context/v1"
	semanticProfileName = "FACIS DCS Semantic Contract Profile"
	semanticProfileV1   = "v1"
	ontologyV1          = "https://w3id.org/facis/dcs/ontology/v1"
	shaclShapesV1       = "https://w3id.org/facis/dcs/shapes/v1"
)

// templateEnvelopeSet holds field names that are set exclusively from the DB row
// (or are structural envelope keys). They are never copied from the inner JSONB
// into the outer envelope to prevent stale values from overriding DB values.
var templateEnvelopeSet = map[string]bool{
	"@context":        true,
	"@id":             true,
	"@type":           true,
	"did":             true,
	"uuid":            true,
	"schemaVersion":   true,
	"documentNumber":  true,
	"templateVersion": true,
	"name":            true,
	"description":     true,
	"createdAt":       true,
	"updatedAt":       true,
	"semanticProfile": true,
	"template_data":   true,
}

// contractEnvelopeSet holds field names that are set exclusively from the DB row
// for contracts. They are not read from the inner JSONB.
var contractEnvelopeSet = map[string]bool{
	"@context":            true,
	"@id":                 true,
	"@type":               true,
	"did":                 true,
	"uuid":                true,
	"contractVersion":     true,
	"state":               true,
	"lifecycleState":      true,
	"name":                true,
	"description":         true,
	"createdAt":           true,
	"updatedAt":           true,
	"validFrom":           true,
	"validUntil":          true,
	"derivedFromTemplate": true,
	"templateVersion":     true,
	"semanticProfile":     true,
	"contractData":        true,
}

// BuildTemplateJSONLD returns the JSON-LD envelope for a ContractTemplate.
//
// dcs:ContractTemplate documents (new format) are returned as-is with
// relational fields (DID, version, dates) merged on top.
//
// Legacy DocumentStructure documents fall back to the promotion-based envelope.
func BuildTemplateJSONLD(template templatedb.ContractTemplate, profile OntologyProfile) (map[string]any, error) {
	// ── New DCS JSON-LD path ──────────────────────────────────────────────────
	if dcs.IsTemplate(template.TemplateData) {
		inner, err := parseJSONB(template.TemplateData)
		if err != nil {
			return nil, fmt.Errorf("parse template_data (dcs): %w", err)
		}
		inner["@id"] = template.DID
		if template.Name != nil {
			inner["dcs:template_title"] = *template.Name
		}
		inner["dcs:version"] = template.Version
		inner["dcs:createdAt"] = template.CreatedAt.UTC().Format(time.RFC3339)
		inner["dcs:updatedAt"] = template.UpdatedAt.UTC().Format(time.RFC3339)
		return inner, nil
	}

	// ── Legacy promotion-based envelope ──────────────────────────────────────
	inner, err := parseJSONB(template.TemplateData)
	if err != nil {
		return nil, fmt.Errorf("parse template_data: %w", err)
	}

	envelope := map[string]any{
		"@context":        profile.ContextURL,
		"@id":             template.DID,
		"@type":           "ContractTemplate",
		"did":             template.DID,
		"templateVersion": template.Version,
		"schemaVersion":   "v1",
		"createdAt":       template.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":       template.UpdatedAt.UTC().Format(time.RFC3339),
		"semanticProfile": buildSemanticProfile(profile),
	}
	if template.DocumentNumber != nil && *template.DocumentNumber != "" {
		envelope["documentNumber"] = *template.DocumentNumber
	}
	if template.Name != nil && *template.Name != "" {
		envelope["name"] = *template.Name
	}
	if template.Description != nil && *template.Description != "" {
		envelope["description"] = *template.Description
	}
	if uuid := uuidURNFromDID(template.DID); uuid != "" {
		envelope["uuid"] = uuid
	}
	templateData := map[string]any{}
	for key, value := range inner {
		switch {
		case templateEnvelopeSet[key]:
		case profile.TemplatePromotedFields[key]:
			envelope[key] = value
		default:
			templateData[key] = value
		}
	}
	envelope["template_data"] = templateData
	return envelope, nil
}

// BuildContractJSONLD returns the JSON-LD envelope for a Contract.
//
// dcs:Contract documents (new format) are returned as-is with relational
// fields merged.  Legacy DocumentStructure contracts go through the
// promotion-based envelope.
func BuildContractJSONLD(contract contractdb.Contract, sourceTemplate templatedb.ContractTemplate, profile OntologyProfile) (map[string]any, error) {
	// ── New DCS JSON-LD path ──────────────────────────────────────────────────
	if dcs.IsContract(contract.ContractData) {
		inner, err := parseJSONB(contract.ContractData)
		if err != nil {
			return nil, fmt.Errorf("parse contract_data (dcs): %w", err)
		}
		inner["@id"] = contract.DID
		inner["dcs:state"] = contract.State
		inner["dcs:version"] = contract.ContractVersion
		inner["dcs:createdAt"] = contract.CreatedAt.UTC().Format(time.RFC3339)
		inner["dcs:updatedAt"] = contract.UpdatedAt.UTC().Format(time.RFC3339)
		if contract.StartDate != nil {
			inner["dcs:validFrom"] = contract.StartDate.UTC().Format(time.RFC3339)
		}
		if contract.ExpDate != nil {
			inner["dcs:validUntil"] = contract.ExpDate.UTC().Format(time.RFC3339)
		}
		return inner, nil
	}

	// ── Legacy DocumentStructure path ─────────────────────────────────────────
	inner, err := parseJSONB(contract.ContractData)
	if err != nil {
		return nil, fmt.Errorf("parse contract_data: %w", err)
	}

	envelope := map[string]any{
		"@context":            profile.ContextURL,
		"@id":                 contract.DID,
		"@type":               "Contract",
		"did":                 contract.DID,
		"contractVersion":     contract.ContractVersion,
		"state":               contract.State,
		"lifecycleState":      semanticLifecycleState(contract.State),
		"createdAt":           contract.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":           contract.UpdatedAt.UTC().Format(time.RFC3339),
		"derivedFromTemplate": sourceTemplate.DID,
		"templateVersion":     sourceTemplate.Version,
		"semanticProfile":     buildSemanticProfile(profile),
	}
	if uuid := uuidURNFromDID(contract.DID); uuid != "" {
		envelope["uuid"] = uuid
	}
	if contract.Name != nil && *contract.Name != "" {
		envelope["name"] = *contract.Name
	}
	if contract.Description != nil && *contract.Description != "" {
		envelope["description"] = *contract.Description
	}
	if contract.StartDate != nil {
		envelope["validFrom"] = contract.StartDate.UTC().Format(time.RFC3339)
	}
	if contract.ExpDate != nil {
		envelope["validUntil"] = contract.ExpDate.UTC().Format(time.RFC3339)
	}
	contractData := map[string]any{}
	for key, value := range inner {
		switch {
		case contractEnvelopeSet[key]:
		case profile.ContractPromotedFields[key]:
			envelope[key] = value
		default:
			contractData[key] = value
		}
	}
	envelope["contractData"] = contractData
	return envelope, nil
}

// buildSemanticProfile builds the semanticProfile descriptor for a given profile.
func buildSemanticProfile(p OntologyProfile) map[string]any {
	return map[string]any{
		"name":     p.Name,
		"version":  p.Version,
		"context":  p.ContextURL,
		"ontology": p.OntologyURL,
		"shapes":   p.ShapesURL,
	}
}

// standardSemanticProfile returns the default FACIS DCS v1 semantic profile descriptor.
// Used by test fixtures that build stored JSONB; the mapper itself uses buildSemanticProfile.
func standardSemanticProfile() map[string]any {
	return buildSemanticProfile(DefaultProfile())
}

// semanticLifecycleState maps a DCS DB contract state to the JSON-LD ontology
// lifecycle state as defined in docs/semantic-ontology/README.md §15.
func semanticLifecycleState(state string) string {
	switch state {
	case "DRAFT":
		return "Draft"
	case "NEGOTIATION":
		return "InNegotiation"
	case "SUBMITTED":
		return "SubmittedForReview"
	case "REVIEWED":
		return "Reviewed"
	case "APPROVED":
		return "Approved"
	case "TERMINATED":
		return "Terminated"
	case "EXPIRED":
		return "Expired"
	default:
		return state
	}
}

// parseJSONB decodes a *datatype.JSON JSONB value into a generic map.
// Returns an empty (non-nil) map when raw is nil or JSON null.
func parseJSONB(raw *datatype.JSON) (map[string]any, error) {
	if raw == nil || !raw.IsNotNullValue() {
		return map[string]any{}, nil
	}
	var result map[string]any
	if err := json.Unmarshal(*raw, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return map[string]any{}, nil
	}
	return result, nil
}

// encodeMap serializes a generic map to a *datatype.JSON value.
func encodeMap(data map[string]any) (*datatype.JSON, error) {
	j, err := datatype.NewJSON(data)
	if err != nil {
		return nil, err
	}
	return &j, nil
}

var uuidHexPattern = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// uuidURNFromDID attempts to extract a UUID URN from a UUID-like segment embedded
// in the DID. Returns an empty string when no UUID segment is present.
func uuidURNFromDID(did string) string {
	if match := uuidHexPattern.FindString(strings.ToLower(did)); match != "" {
		return "urn:uuid:" + match
	}
	return ""
}
