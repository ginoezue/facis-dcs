"""BDD steps for the contract-state-machine-refactor requirement.

Covers the new Offer/Withdraw commands, the extended transition table
(OFFERED, WITHDRAWN, ACTIVE, REVOKED), the C2PA lifecycle mapping for the
new states, and the outbox events emitted by Offer/Withdraw.

These steps intentionally build each precondition state (`Given contract
"<name>" has reached contract state "<state>"`) through the *narrowest*
already-existing endpoint chain rather than depending on other, unrelated
steps that are already broken in this codebase (e.g. the "verify" step used
by `ContractService._prepare_contract_pending_approval`, which targets a
`/contract/verify` route that does not exist in the Goa design). This keeps
a scenario's pass/fail signal attributable to the contract-state-machine
refactor itself.
"""

import base64
import re
import uuid
from pathlib import Path
from urllib.parse import unquote

import requests as _requests
from behave import given, then, when
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding

from steps.support.api_client import (
    contract_approve_url,
    contract_audit_url,
    contract_offer_url,
    contract_peer_action_url,
    contract_retrieve_by_id_url,
    contract_search_url,
    contract_submit_url,
    contract_terminate_url,
    contract_withdraw_url,
    get_with_headers,
    post_json,
    signature_apply_url,
)
from steps.support.services.auth_service import AuthService
from steps.support.services.contract_service import ContractService
from steps.support.services.pdf_service import PDFService


# ---------------------------------------------------------------------------
# Internal helpers
# ---------------------------------------------------------------------------


def _seed_headers(context, name):
    seed = getattr(context, "contract_seed_headers", None) or {}
    if name in seed:
        return seed[name]
    return getattr(context, "headers", None)


def _repo_root() -> Path:
    # This file lives at steps/template_management/<this file>.py.
    return Path(__file__).resolve().parents[2]


def _did_web_to_hostname(did: str) -> str:
    """Mirror identity.DIDWebToHostname (backend/internal/base/identity/did.go)
    so the BDD client can resolve exactly the hostname:port the server itself
    will resolve when it later verifies this signature server-side.
    """
    prefix = "did:web:"
    assert did.startswith(prefix), f"not a did:web identifier: {did}"
    rest = did[len(prefix):]
    host_encoded = rest.split(":", 1)[0]
    assert host_encoded, f"did:web identifier has empty host component: {did}"
    return unquote(host_encoded)


def _dev_signing_key_path(hostname: str) -> Path:
    """Map a did:web hostname (e.g. 'localhost:8991') to the matching
    checked-in dev private key (backend/certs/dev/signing-<port>.key) — the
    same DID/key pairing backend/.env.dev1 (port 8991) and backend/.env.dev2
    (port 8992) use for the local dev-stack.sh instances. Only these two
    known dev ports are supported: this is a self-peer simulation, not a
    generic did:web resolver, and only works because we control the matching
    private key for the instance under test.
    """
    match = re.search(r":(\d+)$", hostname)
    assert match, (
        f"cannot derive a dev signing key for did:web hostname '{hostname}' "
        "(expected '<host>:<port>', e.g. 'localhost:8991')"
    )
    port = match.group(1)
    key_path = _repo_root() / "backend" / "certs" / "dev" / f"signing-{port}.key"
    assert key_path.is_file(), (
        f"no checked-in dev signing key at '{key_path}' for did:web port {port} — "
        "the peer-path self-simulation in this scenario only supports the "
        "checked-in backend/.env.dev1 (8991) / backend/.env.dev2 (8992) dev "
        "identities (backend/certs/dev/did-8991.json, did-8992.json). If this "
        "DCS instance runs under a different DCS_DID (e.g. the Helm/kind BDD "
        "harness, which currently sets no DCS_DID/DCS_PRIVATE_KEY at all — see "
        "docs/anforderung.md), the peer-path part of AC4 cannot be proven this "
        "way and needs re-scoping with the analyst (e.g. grep-gate/manual-drill "
        "for that entry path, or a real two-instance runner)."
    )
    return key_path


def _sign_secret_value_with_dev_key(key_path: Path, secret_value: str) -> bytes:
    """RSA-PSS(SHA-256) signature matching DIDDocument.Sign (backend/internal/
    base/identity/did.go): Go's rsa.SignPSS is called with nil *PSSOptions,
    which defaults the salt length to "auto" (the maximum possible length for
    signing) — i.e. padding.PSS.MAX_LENGTH here. Go's verify side auto-detects
    the salt length regardless, so this is compatible either way.
    """
    private_key = serialization.load_pem_private_key(key_path.read_bytes(), password=None)
    return private_key.sign(
        secret_value.encode(),
        padding.PSS(mgf=padding.MGF1(hashes.SHA256()), salt_length=padding.PSS.MAX_LENGTH),
        hashes.SHA256(),
    )


def _self_peer_action_credentials(context):
    """Simulate a trusted peer by fetching this DCS instance's own did:web
    document (public, unauthenticated GET /.well-known/did.json — see
    backend/design/did.go) and signing a fresh secret with the matching
    checked-in dev private key. The peer action endpoint
    (backend/internal/service/dcs_to_dcs.go Action()) then does a real,
    successful did:web challenge-response verification against this
    instance's own identity, instead of failing on an unresolvable/invalid
    peer hostname before ever reaching the transition-table check.

    This only proves AC4's peer-path claim because the contract under test
    was also created locally on this same instance (Origin == this DID):
    Approver.Handle's single-writer-per-aggregate forwarding check
    (`processData.Origin != localPeer`) is therefore a no-op, and the very
    same `contractstate.ValidateTransition` the UI-API path hits is reached
    directly — see backend/internal/contractworkflowengine/command/approve.go.
    """
    did_resp = _requests.get(
        f"{context.base_url}/.well-known/did.json",
        timeout=context.http_timeout_seconds,
    )
    assert did_resp.status_code == 200, (
        f"could not fetch this instance's own did:web document from "
        f"{context.base_url}/.well-known/did.json (required to simulate a "
        f"trusted peer): {did_resp.status_code} {did_resp.text}"
    )
    from_peer_did = did_resp.json().get("id")
    assert from_peer_did, f"own did.json response has no 'id' field: {did_resp.text}"

    hostname = _did_web_to_hostname(from_peer_did)
    key_path = _dev_signing_key_path(hostname)

    secret_value = str(uuid.uuid4())
    signature = _sign_secret_value_with_dev_key(key_path, secret_value)
    secret_hash = base64.b64encode(signature).decode()

    return from_peer_did, secret_value, secret_hash


def _offer_contract(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    headers = _seed_headers(context, name)
    resp = post_json(context, contract_offer_url(context), {"did": did, "updated_at": updated_at}, headers=headers)
    assert resp.status_code == 200, (
        f"Offer failed while preparing OFFERED state for '{name}': {resp.status_code} {resp.text}"
    )
    ContractService._refresh_contract(context, name)
    return resp


def _withdraw_contract(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    headers = _seed_headers(context, name)
    resp = post_json(context, contract_withdraw_url(context), {"did": did, "updated_at": updated_at}, headers=headers)
    assert resp.status_code == 200, (
        f"Withdraw failed while preparing WITHDRAWN state for '{name}': {resp.status_code} {resp.text}"
    )
    ContractService._refresh_contract(context, name)
    return resp


def _advance_to_submitted(context, name):
    # DRAFT -> NEGOTIATION -> SUBMITTED via the existing, working submit chain
    # (deliberately not routed through Offer: the exact Offer -> Negotiation
    # wiring is an implementation decision left to the implementer; this
    # helper only needs *a* contract sitting in SUBMITTED).
    ContractService._prepare_contract_under_review(context, name)


def _advance_to_reviewed(context, name):
    _advance_to_submitted(context, name)
    did, _ = ContractService._contract_data(context, name)
    reviewer_h = AuthService.get_headers_for_roles(["Contract Reviewer"])
    retrieve = get_with_headers(context, contract_retrieve_by_id_url(context, did), headers=reviewer_h)
    assert retrieve.status_code == 200, retrieve.text
    updated_at = retrieve.json().get("updated_at")
    review_submit = post_json(
        context,
        contract_submit_url(context),
        ContractService._contract_reviewer_submit_payload(context, did, updated_at),
        headers=reviewer_h,
    )
    assert review_submit.status_code == 200, (
        f"Reviewer submit (forward_to=approval) failed while preparing REVIEWED state for "
        f"'{name}': {review_submit.status_code} {review_submit.text}"
    )
    ContractService._refresh_contract(context, name)


def _advance_to_approved(context, name):
    _advance_to_reviewed(context, name)
    did, _ = ContractService._contract_data(context, name)
    approver_h = AuthService.get_headers_for_roles(["Contract Approver"])
    retrieve = get_with_headers(context, contract_retrieve_by_id_url(context, did), headers=approver_h)
    assert retrieve.status_code == 200, retrieve.text
    updated_at = retrieve.json().get("updated_at")
    approve = post_json(
        context, contract_approve_url(context), {"did": did, "updated_at": updated_at}, headers=approver_h
    )
    assert approve.status_code == 200, (
        f"Approve failed while preparing APPROVED state for '{name}': {approve.status_code} {approve.text}"
    )
    ContractService._refresh_contract(context, name)


def _apply_signature(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    signer_h = AuthService.get_headers_for_roles(["Contract Signer"])
    resp = post_json(
        context,
        signature_apply_url(context),
        {"did": did, "signer_did": "did:example:bdd-counterparty-signer", "updated_at": updated_at},
        headers=signer_h,
    )
    assert resp.status_code == 200, (
        f"Signature apply failed while preparing SIGNED state for '{name}': {resp.status_code} {resp.text}"
    )
    ContractService._refresh_contract(context, name)


def _terminate_contract(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    manager_h = AuthService.get_headers_for_roles(["Contract Manager"])
    resp = post_json(
        context,
        contract_terminate_url(context),
        {"did": did, "reason": "BDD setup", "updated_at": updated_at},
        headers=manager_h,
    )
    assert resp.status_code == 200, (
        f"Terminate failed while preparing TERMINATED state for '{name}': {resp.status_code} {resp.text}"
    )
    ContractService._refresh_contract(context, name)


def _reach_state(context, name, state):
    normalized = state.strip().upper()
    if normalized == "DRAFT":
        ContractService._create_contract_in_draft(context, name)
    elif normalized == "OFFERED":
        ContractService._create_contract_in_draft(context, name)
        _offer_contract(context, name)
    elif normalized == "WITHDRAWN":
        ContractService._create_contract_in_draft(context, name)
        _offer_contract(context, name)
        _withdraw_contract(context, name)
    elif normalized == "NEGOTIATION":
        ContractService._create_contract_in_negotiation(context, name)
    elif normalized == "SUBMITTED":
        ContractService._create_contract_in_draft(context, name)
        _advance_to_submitted(context, name)
    elif normalized == "REVIEWED":
        ContractService._create_contract_in_draft(context, name)
        _advance_to_reviewed(context, name)
    elif normalized == "APPROVED":
        ContractService._create_contract_in_draft(context, name)
        _advance_to_approved(context, name)
    elif normalized == "SIGNED":
        ContractService._create_contract_in_draft(context, name)
        _advance_to_approved(context, name)
        _apply_signature(context, name)
    elif normalized == "TERMINATED":
        ContractService._create_contract_in_draft(context, name)
        _advance_to_approved(context, name)
        _terminate_contract(context, name)
    else:
        raise NotImplementedError(
            f"No BDD setup path implemented for target contract state '{state}' — "
            "ACTIVE (deployment/ORCE) and REVOKED (signature revoke) are out of scope "
            "for the contract-state-machine-refactor AC set and are not wired here."
        )


# ---------------------------------------------------------------------------
# Given
# ---------------------------------------------------------------------------


@given('contract "{name}" has reached contract state "{state}"')
def step_given_contract_reached_state(context, name, state):
    _reach_state(context, name, state)
    did, _ = ContractService._contract_data(context, name)
    headers = _seed_headers(context, name)
    retrieve = get_with_headers(context, contract_retrieve_by_id_url(context, did), headers=headers)
    assert retrieve.status_code == 200, retrieve.text
    actual_state = str(retrieve.json().get("state", "")).upper()
    assert actual_state == state.strip().upper(), (
        f"BDD setup could not reach state '{state}' for contract '{name}' "
        f"(this is the expected red signal before the contract-state-machine "
        f"refactor lands): got '{actual_state}'"
    )


# ---------------------------------------------------------------------------
# When
# ---------------------------------------------------------------------------


@when('the initiator offers contract "{name}"')
def step_when_offer_contract(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    headers = _seed_headers(context, name)
    context.requests_response = post_json(
        context, contract_offer_url(context), {"did": did, "updated_at": updated_at}, headers=headers
    )
    if context.requests_response.status_code == 200:
        ContractService._refresh_contract(context, name)


@when('the initiator withdraws contract "{name}"')
def step_when_withdraw_contract(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    headers = _seed_headers(context, name)
    context.requests_response = post_json(
        context, contract_withdraw_url(context), {"did": did, "updated_at": updated_at}, headers=headers
    )
    if context.requests_response.status_code == 200:
        ContractService._refresh_contract(context, name)


@when('contract "{name}" is submitted, reviewed, and approved via the standard workflow')
def step_when_full_approval_workflow(context, name):
    _advance_to_approved(context, name)


@when('the counterparty signer applies a signature to contract "{name}"')
def step_when_apply_signature(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    signer_h = AuthService.get_headers_for_roles(["Contract Signer"])
    context.requests_response = post_json(
        context,
        signature_apply_url(context),
        {"did": did, "signer_did": "did:example:bdd-counterparty-signer", "updated_at": updated_at},
        headers=signer_h,
    )
    if context.requests_response.status_code == 200:
        ContractService._refresh_contract(context, name)


@when('a peer attempts to approve contract "{name}" via the peer action endpoint')
def step_when_peer_attempts_approve(context, name):
    did, updated_at = ContractService._contract_data(context, name)
    # Simulate a trusted, successfully-authenticated peer (see
    # _self_peer_action_credentials docstring) so a 4xx/5xx here can only be
    # the transition-table rejection itself, not a did:web auth failure.
    from_peer_did, secret_value, secret_hash = _self_peer_action_credentials(context)
    payload = {
        "action": "approve",
        "component": "ContractWorkflowEngine",
        "from_peer_did": from_peer_did,
        "payload": {"did": did, "updated_at": updated_at},
        "secret_value": secret_value,
        "secret_hash": secret_hash,
    }
    context.requests_response = post_json(context, contract_peer_action_url(context), payload, headers={})


@when('contract "{name}" is exported and verified as PDF')
def step_when_export_and_verify(context, name):
    did, _ = ContractService._contract_data(context, name)
    export_resp = PDFService.export_contract_pdf(context, did)
    assert export_resp.status_code == 200, (
        f"PDF export failed for contract '{name}': {export_resp.status_code} {export_resp.text}"
    )
    context.requests_response = PDFService.verify_contract_pdf(context, did)


@when('the contract search endpoint is queried with state filter "{state}"')
def step_when_search_by_state(context, state):
    headers = getattr(context, "headers", {})
    context.requests_response = _requests.get(
        contract_search_url(context),
        params={"state": state},
        headers=headers,
        timeout=context.http_timeout_seconds,
    )


# ---------------------------------------------------------------------------
# Then
# ---------------------------------------------------------------------------


@then('the contract "{name}" is in state "{state}"')
def step_then_contract_in_state(context, name, state):
    did, _ = ContractService._contract_data(context, name)
    headers = _seed_headers(context, name)
    retrieve = get_with_headers(context, contract_retrieve_by_id_url(context, did), headers=headers)
    assert retrieve.status_code == 200, retrieve.text
    actual = str(retrieve.json().get("state", "")).upper()
    assert actual == state.strip().upper(), (
        f"Expected contract '{name}' to be in state '{state}', got '{actual}'"
    )


@then("the withdraw request is rejected")
def step_then_withdraw_rejected(context):
    assert context.requests_response.status_code in (400, 404, 409, 422), (
        "Expected withdraw to be rejected once the contract is no longer in a "
        f"pre-approval state, got {context.requests_response.status_code}: "
        f"{context.requests_response.text}"
    )


@then("the request is denied with a client error")
def step_then_denied_client_error(context):
    assert context.requests_response.status_code in (400, 401, 403, 404, 409, 422), (
        "Expected the invalid state transition to be rejected, got "
        f"{context.requests_response.status_code}: {context.requests_response.text}"
    )


@then("the peer action request fails")
def step_then_peer_action_fails(context):
    resp = context.requests_response
    assert resp.status_code != 200, (
        "Expected the invalid transition attempted via the peer action endpoint to "
        f"fail, got 200: {resp.text}"
    )
    # The peer-auth handshake (did:web fetch + eIDAS check + challenge-response
    # verify, see backend/internal/service/dcs_to_dcs.go Action()) is simulated
    # as succeeding (see _self_peer_action_credentials), so a failure here can
    # only honestly evidence AC4's peer-path claim if it is the same
    # contractstate.ValidateTransition rejection the UI-API path hits — not a
    # did:web auth error that happens to also return a non-200.
    body_text = resp.text.lower()
    assert "transition" in body_text or "not allowed" in body_text, (
        "Expected the peer action to fail because of the invalid state "
        "transition itself (backend/internal/contractworkflowengine/datatype/"
        "contractstate ErrInvalidTransition), not a did:web peer-auth error — "
        f"got {resp.status_code}: {resp.text}"
    )


@then('the contract "{name}" has an audit event of type "{event_type}"')
def step_then_contract_has_audit_event(context, name, event_type):
    did, _ = ContractService._contract_data(context, name)
    auditor_h = AuthService.get_headers_for_roles(["Auditor"])
    resp = post_json(context, contract_audit_url(context), {"did": did}, headers=auditor_h)
    assert resp.status_code == 200, f"Audit query failed for contract '{name}': {resp.status_code} {resp.text}"
    events = resp.json()
    assert isinstance(events, list), f"Expected audit response to be a list, got: {events}"
    event_types = [str(e.get("event_type", "")).upper() for e in events]
    assert event_type.upper() in event_types, (
        f"Expected an audit event of type '{event_type}' for contract '{name}', "
        f"got event types: {event_types}"
    )


@then('the C2PA lifecycle_status for contract "{name}" is "{status}"')
def step_then_c2pa_lifecycle_status(context, name, status):
    assert context.requests_response.status_code == 200, (
        f"Verify failed for contract '{name}': {context.requests_response.status_code} "
        f"{context.requests_response.text}"
    )
    body = context.requests_response.json()
    actual = str(body.get("lifecycle_status", "")).lower()
    assert actual == status.lower(), (
        f"Expected C2PA lifecycle_status '{status}' for contract '{name}', got '{actual}': {body}"
    )


@then('the search results include contract "{name}"')
def step_then_search_includes_contract(context, name):
    did, _ = ContractService._contract_data(context, name)
    results = context.requests_response.json()
    assert isinstance(results, list), f"Expected search response to be a list, got: {results}"
    dids = [r.get("did") for r in results]
    assert did in dids, f"Expected contract '{name}' ({did}) in search results, got dids: {dids}"
