package dcstodcs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"digital-contracting-service/internal/base/federation"
	"digital-contracting-service/internal/base/identity"
	"digital-contracting-service/internal/pdfgeneration/provenance"
	qry "digital-contracting-service/internal/processauditandcompliance/query"
)

// Direction names which side of an interaction the trust gate is being
// consulted for, carried verbatim in the policy-endpoint request body
// (ADR-18).
type Direction string

const (
	Inbound  Direction = "inbound"
	Outbound Direction = "outbound"
)

// GateFailureKind distinguishes the agreement-credential check (layer 3a,
// whose failure is retried via sync_fails on the outbound path) from the
// policy-endpoint check (layer 3b, whose failure is always terminal — never
// retried), per ADR-18's architect decision.
type GateFailureKind int

const (
	AgreementFailure GateFailureKind = iota
	PolicyFailure
)

// GateError is returned by TrustGate.Check on any rejection, naming which of
// the two ADR-18 layers rejected the interaction and the peer involved (so a
// caller can anchor an incident report without threading the peer DID
// through separately).
type GateError struct {
	Kind    GateFailureKind
	PeerDID string
	Err     error
}

func (e *GateError) Error() string { return e.Err.Error() }
func (e *GateError) Unwrap() error { return e.Err }

// pdpTimeout bounds every HTTP call the trust gate makes (policy-endpoint
// consult and agreement-credential/did.json fetch): http.DefaultClient has no
// timeout, so a policy endpoint or peer that accepts the connection and then
// never responds (ADR-18 AC10's "silent" fail-closed simulation, or a
// genuinely wedged peer) would otherwise hang the ship/receive attempt
// indefinitely instead of denying — fail-closed requires eventually failing.
const pdpTimeout = 10 * time.Second

// TrustGate implements ADR-18's federation trust gate — the third and final
// trust layer: layer 3a (the peer's self-signed agreement credential must
// verify against its own did.json's dedicated VC key and name this instance's
// own embedded federation rules hash) and layer 3b (this instance's own local
// policy
// endpoint, DCS_TRUST_PDP_URL, must answer 2xx). Consulted on both the
// inbound (PostPdf) and outbound (shipContractPDF) paths. Fail-closed: an
// unset, unreachable, or non-responding policy endpoint denies exactly like
// an explicit deny.
type TrustGate struct {
	PDPURL     string
	HTTPClient *http.Client
}

func (g *TrustGate) httpClient() *http.Client {
	if g.HTTPClient != nil {
		return g.HTTPClient
	}
	return &http.Client{Timeout: pdpTimeout}
}

// Check verifies the peer's agreement credential (layer 3a) then consults
// the local policy endpoint (layer 3b) for one interaction.
func (g *TrustGate) Check(ctx context.Context, peerDID string, direction Direction, contractDID, targetState string) error {
	credential, err := g.verifyAgreementCredential(peerDID)
	if err != nil {
		return &GateError{Kind: AgreementFailure, PeerDID: peerDID, Err: err}
	}
	if err := g.consultPolicyEndpoint(ctx, peerDID, credential, direction, contractDID, targetState); err != nil {
		return &GateError{Kind: PolicyFailure, PeerDID: peerDID, Err: err}
	}
	return nil
}

// verifyAgreementCredential fetches the peer's agreement credential, checks
// its signature against the dedicated VC key published as the SECOND
// verificationMethod in the peer's own did.json, and compares its
// termsOfUse.hash against this instance's own embedded federation rules
// hash.
func (g *TrustGate) verifyAgreementCredential(peerDID string) (json.RawMessage, error) {
	hostname, err := identity.DIDWebToHostname(peerDID)
	if err != nil {
		return nil, fmt.Errorf("agreement credential: %w", err)
	}
	peerDIDDocument, err := identity.FetchDIDDocumentFromHostname(hostname)
	if err != nil {
		return nil, fmt.Errorf("agreement credential: fetch peer did.json: %w", err)
	}
	if len(peerDIDDocument.VerificationMethod) < 2 {
		return nil, fmt.Errorf("agreement credential: peer did.json publishes no dedicated VC verificationMethod")
	}
	vcMethod := peerDIDDocument.VerificationMethod[1]
	vcPub, err := vcMethod.PublicKeyJWK.ECPublicKey()
	if err != nil {
		return nil, fmt.Errorf("agreement credential: peer VC verificationMethod key: %w", err)
	}

	credential, err := fetchAgreementCredential(hostname)
	if err != nil {
		return nil, fmt.Errorf("agreement credential: %w", err)
	}

	var parsed struct {
		Issuer json.RawMessage `json:"issuer"`
		Proof  struct {
			VerificationMethod string `json:"verificationMethod"`
		} `json:"proof"`
		TermsOfUse json.RawMessage `json:"termsOfUse"`
	}
	if err := json.Unmarshal(credential, &parsed); err != nil {
		return nil, fmt.Errorf("agreement credential: decode: %w", err)
	}

	// The credential's issuer is NOT required to equal peerDID verbatim: trust
	// is anchored to the HOSTNAME did:web resolves to (identity.DIDWebToHostname
	// stops at the first ":" after the "did:web:" prefix, exactly like the
	// challenge-response check above it in PostPdf/shipToPeers), not to an
	// exact DID string match — a peer identifier may legitimately carry a
	// path/fragment suffix beyond the host component.
	if _, err := issuerIdentifier(parsed.Issuer); err != nil {
		return nil, fmt.Errorf("agreement credential: %w", err)
	}
	if parsed.Proof.VerificationMethod != vcMethod.ID {
		return nil, fmt.Errorf("agreement credential: proof.verificationMethod %q does not reference the peer's dedicated VC key %q",
			parsed.Proof.VerificationMethod, vcMethod.ID)
	}
	if err := provenance.VerifyDataIntegrityProof(credential, vcPub); err != nil {
		return nil, fmt.Errorf("agreement credential: signature does not verify: %w", err)
	}

	rulesHash, err := termsOfUseHash(parsed.TermsOfUse)
	if err != nil {
		return nil, fmt.Errorf("agreement credential: %w", err)
	}
	if rulesHash != federation.Hash() {
		return nil, fmt.Errorf("agreement credential: federation rules hash %q does not match this instance's own embedded hash %q",
			rulesHash, federation.Hash())
	}

	return credential, nil
}

// consultPolicyEndpoint is layer 3b: the sole peer-authorization authority.
// Any non-2xx response, an unset PDPURL, or an unreachable endpoint all deny
// (fail-closed) — none is distinguished from another.
func (g *TrustGate) consultPolicyEndpoint(ctx context.Context, peerDID string, credential json.RawMessage, direction Direction, contractDID, targetState string) error {
	pdpURL := strings.TrimSpace(g.PDPURL)
	if pdpURL == "" {
		return fmt.Errorf("policy endpoint: DCS_TRUST_PDP_URL is not configured")
	}

	body, err := json.Marshal(map[string]interface{}{
		"peerDID":             peerDID,
		"agreementCredential": credential,
		"direction":           string(direction),
		"contractDID":         contractDID,
		"targetState":         targetState,
	})
	if err != nil {
		return fmt.Errorf("policy endpoint: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pdpURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("policy endpoint: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("policy endpoint: unreachable: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("policy endpoint: denied (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// RecordDenialIncident persists one trust-gate rejection into the audit
// trail (ADR-18: "raises an incident in the audit trail"), regardless of
// which of the two layers (agreement credential or policy endpoint) denied
// the interaction — used by both the inbound (PostPdf) and outbound
// (shipContractPDF) call sites.
func RecordDenialIncident(ctx context.Context, db *sqlx.DB, contractDID string, direction Direction, gateErr *GateError) error {
	reporter := qry.TrustGateDenialReporter{DB: db}
	return reporter.Handle(ctx, denialQry(contractDID, direction, gateErr))
}

// RecordDenialIncidentTxDeduped records a trust-gate rejection in the
// caller's transaction, skipping the insert if an incident for the exact same
// (contract DID, peer DID, direction) was already recorded — a terminal
// policy-endpoint denial
// (PolicyFailure) can be reached more than once for the same underlying
// interaction (ADR-18 AC10: Offer and PDF_REGENERATED both firing a ship
// attempt for the same offer, milliseconds apart), and only the first must
// raise an incident.
func RecordDenialIncidentTxDeduped(ctx context.Context, tx *sqlx.Tx, db *sqlx.DB, contractDID string, direction Direction, gateErr *GateError) error {
	reporter := qry.TrustGateDenialReporter{DB: db}
	return reporter.HandleTxDeduped(ctx, tx, denialQry(contractDID, direction, gateErr))
}

func denialQry(contractDID string, direction Direction, gateErr *GateError) qry.TrustGateDenialQry {
	peerDID, reason := "", ""
	if gateErr != nil {
		peerDID = gateErr.PeerDID
		reason = gateErr.Error()
	}
	return qry.TrustGateDenialQry{
		DID:       contractDID,
		PeerDID:   peerDID,
		Direction: string(direction),
		Reason:    reason,
	}
}

// fetchAgreementCredential fetches a peer's agreement credential via
// /.well-known/dcs-agreement-credential.json, first over https, then
// falling back to http — the same resolution order
// identity.FetchDIDDocumentFromHostname uses for did.json.
func fetchAgreementCredential(hostname string) (json.RawMessage, error) {
	var lastErr error
	for _, scheme := range []string{"https", "http"} {
		url := fmt.Sprintf("%s://%s/.well-known/dcs-agreement-credential.json", scheme, hostname)
		body, err := fetchAgreementCredentialFromURL(url)
		if err == nil {
			return body, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("fetching agreement credential from %s failed: %w", hostname, lastErr)
}

// agreementCredentialFetchClient bounds the peer fetch the same way
// TrustGate.httpClient bounds the policy-endpoint POST (pdpTimeout) — an
// unresponsive peer must fail this out, not hang it.
var agreementCredentialFetchClient = &http.Client{Timeout: pdpTimeout}

func fetchAgreementCredentialFromURL(url string) (json.RawMessage, error) {
	resp, err := agreementCredentialFetchClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s from %s", resp.Status, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading agreement credential: %w", err)
	}
	return body, nil
}

// issuerIdentifier extracts the issuer DID, per W3C VC Data Model 2.0 §4.4
// either a bare string or an object carrying an "id".
func issuerIdentifier(raw json.RawMessage) (string, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}
	var asObject struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &asObject); err == nil && asObject.ID != "" {
		return asObject.ID, nil
	}
	return "", fmt.Errorf("credential carries no issuer")
}

// termsOfUseHash extracts the TrustFrameworkPolicy hash, per W3C VC Data
// Model 2.0 §5.5 termsOfUse may be a single object or an array of objects.
func termsOfUseHash(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("credential carries no termsOfUse")
	}

	var single struct {
		Type string `json:"type"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(raw, &single); err == nil && single.Hash != "" {
		return single.Hash, nil
	}

	var list []struct {
		Type string `json:"type"`
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, entry := range list {
			if entry.Type == "TrustFrameworkPolicy" && entry.Hash != "" {
				return entry.Hash, nil
			}
		}
	}

	return "", fmt.Errorf("credential's termsOfUse carries no TrustFrameworkPolicy hash")
}
