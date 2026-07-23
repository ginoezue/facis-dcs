// Package dcstodcs runs the DCS-to-DCS federation on the sending side: it
// listens for the PDF-regenerated event (and the signature-applied event),
// and ships the contract's PDF — the self-contained wire format carrying the
// machine-readable JSON-LD, the C2PA provenance chain, and any signatures — to
// the counterparty peer, gating every ship on the federation trust gate
// (trustgate.go: agreement credential + local policy endpoint, ADR-18) and
// retrying failed ships from the sync_fails table. A signed contract
// additionally carries the JAdES (DCS-FR-SM-02). No contract state or task
// ledger crosses the boundary: each DCS runs its own workflow/RBAC (ADR-13).
package dcstodcs

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"digital-contracting-service/internal/base/conf"
	"digital-contracting-service/internal/base/datatype/componenttype"
	"digital-contracting-service/internal/base/event"
	"digital-contracting-service/internal/base/identity"
	"digital-contracting-service/internal/base/ipfs"
	"digital-contracting-service/internal/base/jades"
	"digital-contracting-service/internal/contractworkflowengine/datatype/contractstate"
	"digital-contracting-service/internal/contractworkflowengine/datatype/eventtype"
	"digital-contracting-service/internal/contractworkflowengine/db"
	db2 "digital-contracting-service/internal/dcstodcs/db"
	smeventtype "digital-contracting-service/internal/signingmanagement/datatype/eventtype"

	dcstodcs "digital-contracting-service/gen/dcs_to_dcs"

	cloudevent "github.com/cloudevents/sdk-go/v2/event"
	"github.com/jmoiron/sqlx"
	"goa.design/clue/log"
)

type DCSToDCSSynchronizer struct {
	DB          *sqlx.DB
	CRepo       db.ContractRepo
	SRepo       db2.SyncRepository
	IPFSClient  *ipfs.APIClient
	DIDDocument identity.DIDDocument
	TrustGate   TrustGate
}

// shippableStates are the contract states whose PDF is shipped to the
// counterparty (ADR-13): a first offer, each negotiation counter, and the
// signed agreement. Internal states (DRAFT, SUBMITTED, REVIEWED, APPROVED,
// ACTIVE, TERMINATED) stay local — review/approval never cross the boundary.
var shippableStates = map[string]bool{
	contractstate.Offered.String():     true,
	contractstate.Negotiation.String(): true,
	contractstate.Signed.String():      true,
}

func (s *DCSToDCSSynchronizer) StartSynchronizerJob(ctx context.Context, client *event.CloudEventSubClient) {
	syncHandler := func(evt cloudevent.Event) {
		source, err := componenttype.NewComponentType(evt.Source())
		if err != nil {
			log.Errorf(ctx, err, "failed to parse source component type, %s", evt.Source())
			return
		}

		// A PDF is shipped when the regenerator has produced a fresh one
		// (PDF_REGENERATED, content changes: offer/negotiate) or when a
		// signature has been applied (APPLIED_SIGNATURE, which stores the
		// signed PDF directly). shipContractPDF gates on the shippable state.
		switch source {
		case componenttype.ContractWorkflowEngine:
			// Ship when the regenerator produced a fresh PDF (a content or C2PA
			// state change) OR when the contract entered the shippable OFFERED
			// state. An offer is a pure state transition that changes neither the
			// payload hash nor the C2PA lifecycle (DRAFT and OFFERED both map to
			// "draft"), so a contract edited while DRAFT and then offered emits no
			// PDF_REGENERATED — the offer itself must trigger the ship of the
			// already-stored PDF (shipContractPDF still gates on the shippable
			// state, so a non-shippable transition is a no-op).
			if evt.Type() != eventtype.PDFRegenerated.String() && evt.Type() != eventtype.Offer.String() {
				return
			}
		case componenttype.SignatureManagement:
			smType, err := smeventtype.NewEventType(evt.Type())
			if err != nil || smType != smeventtype.Applied {
				return
			}
		default:
			return
		}

		did, err := didFromEvent(evt)
		if err != nil {
			log.Errorf(ctx, err, "could not read did from event %s", evt.Data())
			return
		}
		if err := s.shipContractPDF(ctx, did); err != nil {
			log.Errorf(ctx, err, "failed to ship contract PDF, %s", evt.Data())
		}
	}

	go func() {
		if err := client.Subscribe(syncHandler); err != nil {
			log.Errorf(ctx, err, "could not start syncHandler")
		}
	}()

	go s.startSyncFailScheduler(ctx, conf.SyncFailCronJobTimeOut())
}

func didFromEvent(evt cloudevent.Event) (string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(evt.Data(), &data); err != nil {
		return "", fmt.Errorf("unmarshal event data: %w", err)
	}
	did, ok := data["did"].(string)
	if !ok || did == "" {
		return "", errors.New("event carries no did")
	}
	return did, nil
}

func (s *DCSToDCSSynchronizer) startSyncFailScheduler(ctx context.Context, interval time.Duration) {
	readSyncFails := func() ([]db2.SyncFail, error) {
		tx, err := s.DB.BeginTxx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("could not start transaction: %w", err)
		}
		defer func(tx *sqlx.Tx) {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				log.Printf(ctx, "could not rollback transaction: %v", err)
			}
		}(tx)

		attempts, err := s.SRepo.GetPendingSyncFails(ctx, tx)
		if err != nil {
			return nil, fmt.Errorf("failed to read sync fail entries: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("could not commit transaction: %w", err)
		}
		return attempts, nil
	}

	ticker := time.NewTicker(interval)
	for range ticker.C {
		log.Printf(ctx, "start retrying failed contract PDF ships")
		syncFails, err := readSyncFails()
		if err != nil {
			log.Printf(ctx, "could not read sync fails: %v", err)
			continue
		}
		for _, syncFail := range syncFails {
			if err := s.shipContractPDF(ctx, syncFail.DID); err != nil {
				log.Printf(ctx, "contract PDF ship retry was not successful: %v", err)
			}
		}
	}
}

// shipContractPDF fetches the contract's current PDF and ships it to every
// counterparty peer (ADR-13). A signed contract additionally carries the
// JAdES. A failed ship is recorded in sync_fails for later retry; a clean ship
// clears any prior failure. Non-shippable states and contracts without a PDF
// yet are no-ops.
func (s *DCSToDCSSynchronizer) shipContractPDF(ctx context.Context, did string) error {
	localPeer, err := s.DIDDocument.GetID()
	if err != nil {
		return err
	}

	readTx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	contractData, err := s.CRepo.ReadDataByDID(ctx, readTx, did)
	if err != nil {
		_ = readTx.Rollback()
		return fmt.Errorf("could not read contract %s: %w", did, err)
	}
	pdfState, err := s.CRepo.ReadPDFState(ctx, readTx, did)
	if err != nil {
		_ = readTx.Rollback()
		return fmt.Errorf("could not read PDF state for %s: %w", did, err)
	}
	_ = readTx.Rollback()

	state := string(contractData.State)
	if !shippableStates[state] {
		return nil
	}
	if pdfState.IPFSCID == "" {
		// The contract is shippable but its PDF has not been stored yet: the
		// regenerator compiles it asynchronously, so an offer (a pure state
		// transition) can fire before the CID exists. Never silently drop the
		// ship — record a sync_fail so the DB-backed retry scheduler re-attempts
		// once the CID is written. A dropped ship with no record and no retry is
		// a correctness bug, not merely a timing race.
		return s.recordShipOutcome(ctx, did,
			fmt.Errorf("contract %s is shippable but its PDF is not stored yet; deferring ship to the retry scheduler", did), nil)
	}

	recipients := contractData.Responsible.GetParties()

	pdfResult, err := s.IPFSClient.FetchFile(pdfState.IPFSCID)
	if err != nil || len(pdfResult.Data) == 0 {
		return fmt.Errorf("fetch PDF %s from IPFS for %s: %w", pdfState.IPFSCID, did, err)
	}
	pdfBytes := []byte(pdfResult.Data)

	jadesSignature, err := s.jadesForSignedContract(state, contractData)
	if err != nil {
		return err
	}

	shipError := s.shipToPeers(ctx, localPeer, did, state, pdfBytes, jadesSignature, recipients)

	var gateErr *GateError
	if errors.As(shipError, &gateErr) && gateErr.Kind == PolicyFailure {
		// Unified terminal semantics (ADR-18): a policy-endpoint denial —
		// non-2xx, unset DCS_TRUST_PDP_URL, or unreachable — never gets a
		// sync_fails retry entry, so every attempt is a genuinely new
		// interaction, not a retry of the same denial: exactly one incident
		// per attempt (contrast the agreement-credential dedup below). Any
		// sync_fails row a PRIOR, unrelated failure left behind (e.g. the
		// "PDF not stored yet" deferral, which runs before the trust gate is
		// even consulted) must be cleared here too — otherwise the terminal
		// PDP denial would still sit in the retry queue and keep getting
		// re-attempted forever. Cleared atomically with the incident so a
		// crash between the two can't leave one without the other.
		if err := s.clearSyncFailWithIncident(ctx, did, gateErr); err != nil {
			log.Printf(ctx, "could not clear sync fail entry / record trust gate denial incident for %s: %v", did, err)
		}
		return nil
	}
	return s.recordShipOutcome(ctx, did, shipError, gateErr)
}

// clearSyncFailWithIncident deletes any sync_fails retry entry for did and
// records the terminal policy-endpoint denial incident in the same
// transaction (ADR-18 AC10) — deduped per (did, peer, direction), since a
// single offer's Offer and PDF_REGENERATED events each independently trigger
// a ship attempt a few hundred ms apart and both can hit the same denial.
func (s *DCSToDCSSynchronizer) clearSyncFailWithIncident(ctx context.Context, did string, gateErr *GateError) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf(ctx, "could not rollback transaction: %v", err)
		}
	}(tx)

	if err := s.SRepo.DeleteSyncFailEntry(ctx, tx, did); err != nil {
		return fmt.Errorf("could not delete sync fail entry: %w", err)
	}
	if err := RecordDenialIncidentTxDeduped(ctx, tx, s.DB, did, Outbound, gateErr); err != nil {
		return fmt.Errorf("could not record trust gate denial incident: %w", err)
	}
	return tx.Commit()
}

// jadesForSignedContract returns the JAdES a signed contract's ship carries, or
// an empty string for a proposal ship (ADR-13, DCS-FR-SM-02).
func (s *DCSToDCSSynchronizer) jadesForSignedContract(state string, contractData *db.Contract) (string, error) {
	if state != contractstate.Signed.String() {
		return "", nil
	}
	contractDocBytes := []byte(`{}`)
	if contractData.ContractData != nil && contractData.ContractData.IsNotNullValue() {
		contractDocBytes = []byte(*contractData.ContractData)
	}
	payload, err := jades.BuildContractPayload(contractData.DID, contractData.ContractVersion, contractDocBytes)
	if err != nil {
		return "", fmt.Errorf("build JAdES payload for %s: %w", contractData.DID, err)
	}
	signature, err := jades.Sign(&s.DIDDocument, payload)
	if err != nil {
		return "", fmt.Errorf("JAdES-sign %s: %w", contractData.DID, err)
	}
	return signature, nil
}

func (s *DCSToDCSSynchronizer) shipToPeers(ctx context.Context, localPeer, did, state string, pdfBytes []byte, jadesSignature string, recipients []string) error {
	for _, peer := range recipients {
		if peer == localPeer {
			continue
		}
		if err := s.TrustGate.Check(ctx, peer, Outbound, did, state); err != nil {
			return err
		}
		hostname, err := identity.DIDWebToHostname(peer)
		if err != nil {
			return err
		}
		secretValue := rand.Text()
		secretHash, err := s.DIDDocument.Sign([]byte(secretValue))
		if err != nil {
			return err
		}
		client := NewDCSToDCSHttpClient(hostname)
		if _, err := client.PostPdf(ctx, &dcstodcs.DCSToDCSContractPdfRequest{
			FromPeerDid:    localPeer,
			ContractIri:    did,
			Pdf:            pdfBytes,
			SecretValue:    secretValue,
			SecretHash:     secretHash,
			JadesSignature: &jadesSignature,
		}); err != nil {
			return err
		}
	}
	return nil
}

// recordShipOutcome persists a ship attempt's sync_fails side effect and,
// for an agreement-credential trust-gate failure (gateErr != nil), records
// exactly one incident for it. The sync_fails row this contract's ship
// attempts share can be created for an UNRELATED reason first (e.g. the PDF
// not being stored yet, gateErr == nil that time) and only turn into an
// actual gate failure on a later retry — "was this call the one that
// inserted the row" is therefore NOT the right dedup signal (it would miss
// the incident entirely in that interleaving); CreateOrUpdateSyncFailEntry's
// own gate_incident_recorded latch is, and it reports true at most once
// per entry regardless of how many times gate failures do or don't recur.
func (s *DCSToDCSSynchronizer) recordShipOutcome(ctx context.Context, did string, shipError error, gateErr *GateError) error {
	tx, err := s.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not start transaction: %w", err)
	}
	defer func(tx *sqlx.Tx) {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf(ctx, "could not rollback transaction: %v", err)
		}
	}(tx)

	shouldRecordIncident := false
	if shipError != nil {
		shouldRecordIncident, err = s.SRepo.CreateOrUpdateSyncFailEntry(ctx, tx, did, gateErr != nil)
		if err != nil {
			return fmt.Errorf("could not create or update sync fail entry: %w", err)
		}
	} else if err := s.SRepo.DeleteSyncFailEntry(ctx, tx, did); err != nil {
		return fmt.Errorf("could not delete sync fail entry: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	if shouldRecordIncident && gateErr != nil {
		if incidentErr := RecordDenialIncident(ctx, s.DB, did, Outbound, gateErr); incidentErr != nil {
			log.Printf(ctx, "could not record trust gate denial incident for %s: %v", did, incidentErr)
		}
	}
	return shipError
}
