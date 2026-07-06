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
import hashlib

import requests as _requests
from behave import given, then, when

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
    secret_value = "bdd-peer-secret"
    secret_hash = base64.b64encode(hashlib.sha256(secret_value.encode()).digest()).decode()
    payload = {
        "action": "approve",
        "component": "ContractWorkflowEngine",
        "from_peer_did": "did:web:bdd-peer.invalid",
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
    assert context.requests_response.status_code != 200, (
        "Expected the invalid transition attempted via the peer action endpoint to "
        f"fail, got 200: {context.requests_response.text}"
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
