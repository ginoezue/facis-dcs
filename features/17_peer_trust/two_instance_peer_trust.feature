# Two-instance peer trust: the federation agreement credential + local
# policy endpoint (PDP), and cross-instance replication.
#
# ADR-18 (docs/adr-18-federation-agreement-credential.md) replaces the third
# trust layer this file used to describe as a static `trusted_peers`
# allowlist with two layers: (3a) each peer's self-signed agreement
# credential (W3C VC, published at
# GET /.well-known/dcs-agreement-credential.json) and (3b) a local,
# per-instance policy endpoint (PDP) consulted on every inbound/outbound
# interaction, fail-closed. The `trusted_peers` table, its `DCS_TRUSTED_PEERS`
# seeding, and `CheckForUntrustedPeers` are to be removed entirely (ADR-18
# decision item 4) — as of this writing that removal has not landed yet
# (backend/internal/service/dcs_to_dcs.go's PostPdf still calls the OLD
# CheckForUntrustedPeers), so every ADR-18 scenario below is expected to fail
# red; that is the point of writing them ahead of the implementation.
#
# The untrusted-peer single-instance-testable scenarios use a syntactically
# distinct did:web identity that resolves, hostname-wise, to THIS SAME
# instance's own dev key and (per ADR-18) its own
# /.well-known/dcs-agreement-credential.json — see
# steps/peer_trust/dcs_peer_trust_steps.py for why this is honest evidence
# specifically for the agreement-credential/PDP gate, not just "any"
# rejection, and for which sub-cases (missing credential; PDP unreachable)
# are honestly simulatable this way versus which are not (signature-invalid
# or hash-mismatched credentials — flagged as open points at their own
# scenarios below).
#
# The @two-instance scenarios need a second real DCS process and
# BDD_DCS_BASE_URL_A/_B rather than the single-instance BDD_DCS_BASE_URL
# (locally via dev-stack2.sh, in CI via the dcs-a/dcs-b Helm releases).
#
# The AC7-AC10 PDP-gate scenarios drive the shared orce Node-RED flow's
# control surface (POST/GET {BDD_TRUST_PDP_CONTROL_URL}/trust-pdp/mode|last,
# default http://localhost:18880, the harness's own orce port-forward) —
# both DCS instances are wired with DCS_TRUST_PDP_URL=http://dcs-orce:1880/
# trust-pdp — see steps/peer_trust/dcs_trust_pdp_steps.py's module docstring.

@NFR-BR-08
Feature: Two-instance peer trust — federation agreement credential, PDP gate, and cross-instance replication

  # ---------------------------------------------------------------------
  # AC1 / AC2: the agreement credential itself (single-instance).
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC1
  Scenario: The agreement credential names this instance's DID as issuer and its rules by policy ID and hash
    When I fetch this instance's own agreement credential
    Then the agreement credential is a W3C Verifiable Credential whose issuer is this instance's own DID
    And the agreement credential's termsOfUse names the federation rules by policyId and hash

  @REQ-fed-agreement-AC2
  Scenario: The agreement credential's signature verifies against the instance's own published VC key
    Given I fetch this instance's own agreement credential
    Then the credential's proof verifies against the second verificationMethod key published in this instance's own did.json

  # ---------------------------------------------------------------------
  # AC3: both instances of the same build agree — same rules hash, and an
  # offer replicates end to end once both sides' PDPs allow it.
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC3 @two-instance
  Scenario: Both instances of the same build publish the same federation rules hash
    Given instance A and instance B are both running and trust each other
    Then instance A and instance B's agreement credentials name the identical federation rules hash

  @NFR-BR-08 @REQ-fed-agreement-AC3 @two-instance
  Scenario: A contract offered on instance A appears on its counterparty B
    Given instance A and instance B are both running and trust each other
    When the initiator on instance A creates and offers a contract with instance B as counterparty
    Then the contract appears on instance B in state OFFERED within a few seconds

  # ---------------------------------------------------------------------
  # AC4: PostPdf rejects a peer with a missing agreement credential. The
  # shipping peer is the orce trust-PDP flow's synthetic-peer route
  # (deployment/helm/charts/orce/flows/): it mirrors this instance's own
  # did.json (layers 1/2 genuinely verify) but its own agreement-credential
  # endpoint deliberately 404s — see
  # steps/peer_trust/dcs_peer_trust_steps.py's
  # _orce_synthetic_peer_did/_orce_synthetic_peer_credentials. Replaces an
  # earlier "sign with our own key, resolve back to our own now-implemented
  # endpoint" technique that stopped being honest once that endpoint started
  # returning 200 instead of 404.
  #
  # OPEN POINT: the "signature-invalid" half of AC4 ("fehlendem/
  # signatur-ungültigem Agreement-Credential") is still NOT covered by a
  # scenario here — the synthetic-peer route only produces a MISSING
  # credential (404), not a present-but-badly-signed one. Flagged for the
  # analyst/architect rather than forced into a fake scenario.
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC4
  Scenario: PostPdf rejects a peer that publishes no agreement credential at all
    Given a cryptographically valid peer identity
    And that peer publishes no agreement credential
    And contract "AC4 Missing Credential" exists locally, created by this instance
    When that peer ships contract "AC4 Missing Credential"'s PDF to this instance's PostPdf endpoint
    Then the PDF is rejected because the peer's agreement credential does not verify

  # ---------------------------------------------------------------------
  # AC5 (@two-instance): sender-side refusal to ship towards a peer whose
  # agreement credential is VALIDLY SIGNED but names a deliberately WRONG
  # rules hash — the SECOND orce static synthetic peer
  # (step_given_counterparty_synthetic_peer_mismatched_hash,
  # did:web:dcs-orce-mismatch%3A1880 by default). Reformulated from an
  # earlier inbound/PostPdf/two-BUILD-variant draft to the same
  # outbound/sender-side shape AC6 already uses (reusing its Then-steps):
  # the hash-comparison gate condition is exercised identically either way,
  # honestly, without needing a second deployment BUILD (only a second
  # static peer identity, which now exists).
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC5 @two-instance
  Scenario: An offer towards a peer whose agreement credential names a different rules hash is refused
    Given instance A and instance B are both running and trust each other
    And the counterparty is a synthetic peer whose agreement credential names a different rules hash
    When the initiator on instance A creates and offers a contract with instance B as counterparty
    Then instance A does not ship the PDF and a sync_fails retry entry exists for the cross-instance contract
    And an incident report naming instance A's refusal to ship is recorded in instance A's audit trail

  # ---------------------------------------------------------------------
  # AC6 (@two-instance): sender-side refusal to ship without a valid,
  # hash-matching peer credential — sync_fails + incident on the SENDER's
  # own side. The counterparty is overridden to the orce synthetic-peer
  # route (same identity as AC4's, see
  # step_given_counterparty_synthetic_peer_no_credential), whose
  # agreement-credential endpoint deliberately 404s — real instance B's own
  # endpoint is now implemented and would actually verify, defeating the
  # point of this scenario.
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC6 @two-instance
  Scenario: Instance A refuses to ship without a valid agreement credential from instance B
    Given instance A and instance B are both running and trust each other
    And the counterparty is a synthetic peer with no valid agreement credential
    When the initiator on instance A creates and offers a contract with instance B as counterparty
    Then instance A does not ship the PDF and a sync_fails retry entry exists for the cross-instance contract
    And an incident report naming instance A's refusal to ship is recorded in instance A's audit trail

  # ---------------------------------------------------------------------
  # AC7: the local policy endpoint (PDP) is consulted on every interaction,
  # in- and outbound; a 2xx response lets it proceed. Single-instance, orce
  # trust-PDP flow in "allow" mode.
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC7
  Scenario: An inbound PostPdf interaction consults the local policy endpoint before proceeding
    Given the local policy endpoint (PDP) is running and allows every request
    And a cryptographically valid peer identity
    And contract "AC7 Inbound PDP Consult" exists locally, created by this instance
    When that peer ships contract "AC7 Inbound PDP Consult"'s PDF to this instance's PostPdf endpoint
    Then the policy endpoint was consulted for this interaction

  @REQ-fed-agreement-AC7
  Scenario: An outbound ship consults the local policy endpoint before proceeding
    Given the local policy endpoint (PDP) is running and allows every request
    And contract "AC7 Outbound PDP Consult" exists locally, offered to a peer counterparty, created by this instance
    When this instance ships contract "AC7 Outbound PDP Consult"'s PDF towards its peer counterparty
    Then the policy endpoint was consulted for this interaction

  # ---------------------------------------------------------------------
  # AC8: PDP non-2xx -> denial + exactly one incident (Deny is TERMINAL per
  # the architect's decision — no sync_fails retry entry for a Deny). The
  # incident count is scoped to THIS scenario's own contract DID — an
  # unscoped global count is unreliable in a shared long-lived environment
  # where earlier/orphaned denials can already have raised unrelated
  # incidents.
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC8
  Scenario: A denying policy endpoint rejects the interaction with exactly one incident and no retry
    Given the local policy endpoint (PDP) is running and denies every request
    And a cryptographically valid peer identity
    And contract "AC8 PDP Deny" exists locally, created by this instance
    When that peer ships contract "AC8 PDP Deny"'s PDF to this instance's PostPdf endpoint
    Then the interaction is denied and exactly one incident is recorded in the audit trail for contract "AC8 PDP Deny"
    And no sync_fails retry entry exists for contract "AC8 PDP Deny"

  # ---------------------------------------------------------------------
  # AC9: the PDP request body carries the documented fields. (Checked via
  # the orce control flow's GET /trust-pdp/last, which reports the most
  # recently received request — not an exact request COUNT, unlike the
  # retired local-stub version of this pack.)
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC7 @REQ-fed-agreement-AC9
  Scenario: The policy endpoint receives peerDID, agreementCredential, direction, contractDID, and targetState
    Given the local policy endpoint (PDP) is running and allows every request
    And a cryptographically valid peer identity
    And contract "AC9 PDP Body Shape" exists locally, created by this instance
    When that peer ships contract "AC9 PDP Body Shape"'s PDF to this instance's PostPdf endpoint
    Then the policy endpoint recorded a request naming the peer, the agreement credential, the direction, the contract, and the target state

  # ---------------------------------------------------------------------
  # AC10: DCS_TRUST_PDP_URL unreachable -> fail-closed denial + incident.
  #
  # Unified terminal semantics (architect decision): EVERY PDP failure mode —
  # non-2xx deny, unset DCS_TRUST_PDP_URL, or an unreachable endpoint — is
  # equally terminal: the interaction is rejected, exactly one incident is
  # recorded, and NO sync_fails retry entry is created for a PDP-caused
  # rejection (unlike AC6's agreement-credential-caused refusal, which DOES
  # get a sync_fails entry — that is a different gate). There is therefore no
  # "transient, keeps retrying" sub-case to distinguish here; this scenario's
  # Then-steps are identical in shape to AC8's deny scenario.
  #
  # OPEN POINT: the OTHER AC10 sub-case — DCS_TRUST_PDP_URL entirely UNSET —
  # is not covered here. It is a backend-startup-time env var fixed for the
  # life of the running process; exercising it needs a dedicated deployment
  # variant with the var deliberately absent (no analogue to
  # tests/bdd/Makefile's single-purpose kind_up_audit stack exists for this
  # yet). Flagged rather than silently only testing the reachable sub-case
  # under this AC's name.
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC10
  Scenario: An unreachable policy endpoint fails closed with exactly one incident and no retry
    Given the local policy endpoint (PDP) is not reachable
    And contract "AC10 PDP Unreachable" exists locally, offered to a peer counterparty, created by this instance
    When this instance ships contract "AC10 PDP Unreachable"'s PDF towards its peer counterparty
    Then the outbound ship attempt is denied and exactly one incident is recorded in the audit trail for contract "AC10 PDP Unreachable"
    And no sync_fails retry entry exists for contract "AC10 PDP Unreachable"

  # ---------------------------------------------------------------------
  # AC11 (@two-instance): the wired default Node-RED flow
  # (deployment/helm/charts/orce/flows/trust-pdp-flow.json) answers 200 OK
  # and mutual trust runs through it. Still gated behind an explicit Given
  # (see step_given_default_pdp_flow_wired) whose env var the coordinator
  # sets once a given harness run has confirmed both instances'
  # DCS_TRUST_PDP_URL wiring is live, rather than assuming the flow file's
  # mere presence in the chart means it.
  # ---------------------------------------------------------------------

  @REQ-fed-agreement-AC11 @two-instance
  Scenario: Mutual trust between instances runs through the default Node-RED policy flow
    Given instance A and instance B are both running and trust each other
    And the default trust-PDP Node-RED flow is wired on both instances
    When the initiator on instance A creates and offers a contract with instance B as counterparty
    Then the contract appears on instance B in state OFFERED within a few seconds
