package service

import (
	"context"
	"encoding/json"

	"digital-contracting-service/internal/base/identity"

	didservice "digital-contracting-service/gen/did_service"
)

type DIDSrv struct {
	DIDocument          identity.DIDDocument
	AgreementCredential []byte
	FederationRules     []byte
}

// NewDIDService takes the already-built (and signed) agreement credential
// bytes (federation.BuildAgreementCredential, built once at startup with the
// HSM VC signer — ADR-18) and the embedded federation rules document
// (federation.Rules()), so this request handler stays a pure read of
// precomputed state.
func NewDIDService(didDocument identity.DIDDocument, agreementCredential, federationRules []byte) (didservice.Service, error) {
	return &DIDSrv{
		DIDocument:          didDocument,
		AgreementCredential: agreementCredential,
		FederationRules:     federationRules,
	}, nil
}

func (s DIDSrv) GetServiceDID(ctx context.Context) (res any, err error) {
	return s.DIDocument.GetDIDContent(), nil
}

// GetAgreementCredential serves this instance's self-signed federation
// agreement credential (ADR-18).
func (s DIDSrv) GetAgreementCredential(ctx context.Context) (res any, err error) {
	var content map[string]interface{}
	if err := json.Unmarshal(s.AgreementCredential, &content); err != nil {
		return nil, didservice.MakeInternalError(err)
	}
	return content, nil
}

// GetFederationRules serves the federation rules document embedded in this
// instance's binary (ADR-18); its content hash is the value the agreement
// credential's termsOfUse.hash names.
func (s DIDSrv) GetFederationRules(ctx context.Context) (res []byte, err error) {
	return s.FederationRules, nil
}
