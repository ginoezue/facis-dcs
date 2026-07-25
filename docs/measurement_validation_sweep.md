# Measurement and Validation Sweep

Status: Working draft -- extraction and traceability only; no implementation claim

## Purpose and authority

This document inventories every measurement vector and every product-validation path that can be
derived from the current requirements sources. It deliberately does not treat code, feature tags,
tests, comments, READMEs, or an `implementation_status` value as proof that a requirement is met.

Source precedence:

1. `docs/requirements_interpretation.md`, especially `effective_requirement`,
   `effective_acceptance_criteria`, `interpretation_status`, and `implementation_decision`.
2. `docs/SRS_FACIS_DCS.txt` as supplementary context and as the source of the explicit
   `Measurement` / `Verification Method` vectors.
3. Repository artifacts only as traceability candidates to be independently verified later.

The sweep separates three concepts that the SRS sometimes places close together:

- **Measurement vector**: an observation or metric to collect.
- **Validation path**: product behavior that accepts an input, performs checks, and returns or
  enforces an outcome.
- **Verification method**: the future evidence needed to demonstrate the requirement.

## Extraction rules

- Preserve one row per independently falsifiable vector, even when one requirement lists several
  metrics or checks.
- Prefer the effective requirement over SRS wording on conflict.
- Mark missing targets, fixtures, policies, rule packs, or external dependencies explicitly.
- Treat `Done` only as planning metadata. It is not evidence.
- Record positive, negative, boundary, stale-state, concurrency, and external-failure cases.
- Do not silently discard an SRS-only vector. Classify it as supplementary or conflicting.

## A. Explicit SRS measurement vectors

The SRS contains 32 NFR entries with an explicit `Measurement` field. None of those 32 fields
defines a numeric acceptance threshold. Some parent requirements do contain a binary normative
criterion (for example TLS 1.3), but the measurement field still lacks a sample size, test profile,
environment, pass threshold, or observation window.

| ID | Measurement vector(s) | SRS verification method | Target gap / boundary |
|---|---|---|---|
| DCS-NFR-PER-01 | Response time under load; throughput | Documentation review; testing | Load model, percentiles, concurrency and pass limits absent |
| DCS-NFR-PER-02 | Scalability results; supported users; supported connections | Documentation review; testing | Baseline, growth curve and acceptable degradation absent |
| DCS-NFR-PER-03 | Uptime percentage; MTTR | Documentation review; testing | Availability target and observation window absent |
| DCS-NFR-SF-01 | Recovery success rate; MTTR | Documentation review; testing | Failure classes, state-loss tolerance and targets absent |
| DCS-NFR-SF-02 | Remote-access security results; penetration findings | Documentation review; testing | Applicable remote-admin surface and acceptable findings absent |
| DCS-NFR-SF-03 | RTO; RPO | Documentation review; testing | Required RTO/RPO values and disaster scenarios absent |
| DCS-NFR-SEC-01 | TLS version compliance; security-audit results | Documentation review; testing | Normative protocol rule exists; scan scope and zero-tolerance rule not stated in measurement |
| DCS-NFR-SEC-02 | Standards compliance; cryptographic strength | Documentation review; testing | Authoritative standard/version and allowed algorithm catalogue absent |
| DCS-NFR-SEC-03 | Authentication success rate; unauthorized attempts | Documentation review; testing | Success-rate target is ambiguous; denial/logging targets absent |
| DCS-NFR-SEC-04 | Configuration-integrity verification logs | Documentation review; testing | Covered configuration set and expected log evidence absent |
| DCS-NFR-SEC-05 | Integrity-test results; detected tampering attempts | Documentation review; testing | Assets, attacks, detection target and remote-attestation scope absent |
| DCS-NFR-SEC-06 | Secure-storage compliance tests; key-access logs | Documentation review; testing | Required hardware class and acceptable access pattern absent |
| DCS-NFR-SEC-07 | Test coverage; security vulnerabilities found | Documentation review | Coverage type/threshold and acceptable vulnerability severity absent |
| DCS-NFR-SEC-08 | Number of data breaches | Documentation review; testing | Observation window and preventive test proxy absent |
| DCS-NFR-SEC-09 | Log availability; detected anomalies; uptime | Documentation review; testing | Retention, completeness, anomaly and uptime targets absent |
| DCS-NFR-SEC-10 | Integrity hash checks; detected modifications | Documentation review; testing | Protected objects, cadence and detection target absent |
| DCS-NFR-SEC-11 | Detected anomalies; MTTD | Documentation review; testing | Incident corpus and MTTD target absent |
| DCS-NFR-SEC-12 | Configuration-compliance score | Documentation review; source review | Benchmark/profile and minimum score absent |
| DCS-NFR-SEC-13 | Completed secure-deletion tasks | Documentation review; testing | Data classes, deletion semantics and completion target absent |
| DCS-NFR-SEC-14 | Encryption coverage percentage | Documentation review; testing | Data inventory, key handling and required percentage absent |
| DCS-NFR-SEC-15 | Security issues found in reviews | Documentation review; source review | Counting findings rewards detection, not remediation; severity gate absent |
| DCS-NFR-SEC-16 | Successful federated logins | Documentation review; testing | Provider matrix, negative cases and pass ratio absent |
| DCS-NFR-SEC-17 | Successful secure-boot validations | Documentation review; testing | Platform boundary and expected failure behavior absent |
| DCS-NFR-SEC-18 | Policy-aligned disclosures; consented M2M transactions; consent-audit results | Documentation review; testing; consent logs | Data-minimization oracle, consent validity and target absent |
| DCS-NFR-SQ-01 | Code-review/audit results; documentation completeness | Documentation review; source review | Style rules, dead-code policy and completeness metric absent |
| DCS-NFR-SQ-02 | Build success rate; deployment consistency | Documentation review; testing | Platforms, repetitions and target absent |
| DCS-NFR-SQ-03 | Container deployment/execution; orchestrator compatibility; CI/CD integration | Documentation review; deployment testing | Required environment matrix and pass criteria absent |
| DCS-NFR-SQ-04 | Privacy-impact assessment results | Documentation review | Required method, approver and acceptable residual risk absent |
| DCS-NFR-SQ-05 | Legally recognized signed transactions | Documentation review; testing | Legal profile, trust anchors and recognition oracle absent |
| DCS-NFR-SQ-06 | Successfully integrated third-party systems | Documentation review; testing | Required systems/protocols and semantic interoperability criteria absent |
| DCS-NFR-SQ-07 | WCAG compliance; usability results | Documentation review; testing | WCAG version/level and usability thresholds absent |
| DCS-NFR-SQ-08 | Working FACIS orchestration/XFSC integration | Documentation review; testing | Effective requirement may permit adapters; deployed component/profile absent |

### A.1 C2PA measurement vectors

These ten vectors are SRS-only supplementary requirements: no corresponding authoritative entries
exist in `requirements_interpretation.md`. They remain in the sweep because they are concrete and do
not automatically conflict with the effective requirements. Their applicability must be decided rather
than inferred from code or feature tags.

| ID | Measurement and target | Required verification | Mandatory cases / failure cases |
|---|---|---|---|
| DCS-OR-C2PA-001 | Valid C2PA manifest on 100% of contract PDFs | Tool validation; documentation review | Missing, malformed, unsigned/untrusted and hash-mismatched manifests |
| DCS-OR-C2PA-002 | 100% pass both PDF-signature and C2PA verification after update | Sign -> append -> verify; PDF/C2PA tools | Embedded and remote manifest; multiple increments; corrupted increment |
| DCS-OR-C2PA-003 | 100% coverage of seven states and all required fields | Schema validation; manifest unit tests | draft, active, amended, suspended, terminated, expired, replaced; every required field missing/invalid |
| DCS-OR-C2PA-004 | 100% lifecycle events have matching VC | VC signature verification; VC/C2PA cross-check | Contract/file/status/reason/time mismatch; missing/revoked/untrusted VC |
| DCS-OR-C2PA-005 | Approved status appears in list within <= 5 minutes | Integration test; timestamp comparison | Suspension, termination, stale/cache/outage and clock-boundary cases |
| DCS-OR-C2PA-006 | Correct banner for 100% of cases | Automated verifier; manual UX check | Independently check PDF signature, C2PA, VC and status list; no aggregate false-positive |
| DCS-OR-C2PA-007 | Key and PoA policy exists; two successful rotations/year | Policy/drill review; key-ID audit | Untrusted/rotated/revoked key; absent/expired/out-of-scope PoA |
| DCS-OR-C2PA-008 | 100% verification after embedded metadata stripping | Strip -> verify; remote-fetch logs | Missing remote manifest, unavailable endpoint, wrong contract binding and tampered remote data |
| DCS-OR-C2PA-009 | Trusted timestamp and audit entry for 100% of events | Log review; TSA verification | Missing/invalid/untrusted timestamp; actor/reason omission; audit-chain tampering |
| DCS-OR-C2PA-010 | Zero PDFs lose legal-signature validity after C2PA updates | Signature validation before/after | Each supported signature profile, lifecycle update order and repeated updates |

## B. Authoritative product-validation paths

The following paths are derived from effective requirements. A row is a behavioral obligation, not a
claim that a single endpoint or existing test covers it.

| Path | Authoritative source(s) | Input and operation | Required outcome | Required negative/boundary coverage | Status / gap |
|---|---|---|---|---|---|
| VAL-01 Template legal/content-policy validation | DCS-FR-TR-07 | Persisted root plus immediate component snapshots; domain policy/rule pack | Findings cover composition-wide content while root-only rules stay root-scoped | Malformed root/component remains a finding; stale repository component must not replace snapshot; no recursive snapshot walk | Ongoing; domain rule packs remain open |
| VAL-02 Template compliance and integrity verification | DCS-FR-TR-20 | Read-only verify request for retrieved template and persisted snapshots | Reproducible integrity/compliance report with component-origin paths | Reusable component changes after composition; standalone component; nested snapshot boundary | Ongoing |
| VAL-03 Direct template dependency validation | DCS-FR-TR-26 | Direct DIDs at selection and create/update, inside persistence transaction | Every reference exists, is COMPONENT, reusable, non-self and acyclic; valid write commits | Missing/malformed/wrong type/wrong state/self/cycle; state changes after selection; failure leaves persistence unchanged | Marked Done, evidence still required |
| VAL-04 Contract assembly validation | DCS-FR-CWE-03 | Reusable clauses/templates plus contract metadata and content | Structure, required metadata and content logic pass before assembled result proceeds | Missing/duplicate/incompatible components; malformed metadata; contradictory logic | Ongoing; criteria underspecified |
| VAL-05 Human contract review validation | DCS-FR-CWE-14, DCS-FR-CWE-25, DCS-IR-CWE-05 | Submitted negotiated contract, reviewer identity/comments | Assigned reviewer can inspect, validate, comment and produce routed status change | Unauthorized/unassigned reviewer; stale version; rejection/comments; concurrent update | Ongoing; validation oracle underspecified |
| VAL-06 External API action validation | DCS-FR-CWE-28 | Authenticated create/update/query action | Action, role, state transition and rate limit are validated | Malformed body, forbidden action, invalid transition, replay/rate limit | Ongoing; action-rule catalogue absent |
| VAL-07 Signer identity/PoA credential validity | DCS-FR-SM-03 | Identity VC and optional PoA VC | Credential is valid, verifiable and from recognized authority | Missing/expired/revoked/malformed/untrusted credential; PoA required but absent | Ongoing; effective text is truncated after "The syste" and needs correction |
| VAL-08 Counterparty delegation-chain verification | DCS-FR-SM-04 | Counterparty PoA chain and trust anchors | Complete, valid, traceable delegation to signer | Broken/cyclic/expired/revoked/out-of-scope delegation; untrusted anchor | Ongoing |
| VAL-09 W3C/eIDAS credential verification | DCS-FR-SM-05 | Presented identity/PoA credentials | Format, signature, issuer/trust and semantic profile validate | Unsupported proof/data model, signature failure, issuer/status outage | Ongoing; exact profiles absent |
| VAL-10 Signature workflow completion validation | DCS-FR-SM-13 | Workflow definition and signer-step states | Completion only after order, deadlines and dependencies are satisfied | Missing/failed/retried/out-of-order/late steps; parallel signer boundary | Not Started |
| VAL-11 Cryptographically validated retrieval for signing | DCS-FR-SM-15 | Retrieved contract, expected identity/hash, signer authorization | Correct immutable contract delivered and retrieval logged | Hash/identity mismatch, altered/stale document, unauthorized signer, logging failure | Not Started |
| VAL-12 Applied-signature validation | DCS-FR-SM-18 | Signed contract, credential status, trust material and timestamp | Separate results for credential status, cryptographic integrity and timestamp; exportable report | Tampered document/signature, revoked/unknown credential, invalid/untrusted/expired timestamp, dependency outage | Not Started |
| VAL-13 Signature revocation invalidation | DCS-FR-SM-20 | Credential or organizational revocation event | Revocation logged; associated contract invalid until re-signing | Duplicate/out-of-order event, partial propagation, re-signing transition | Ongoing |
| VAL-14 Signature-policy compliance | DCS-FR-SM-21 | Signature type, credential status, signer role and policy | Policy violations are explicitly flagged | SES/AES/QES mismatch, role/PoA mismatch, stale status, ambiguous policy | Not Started; policy profiles absent |
| VAL-15 MR/HR synchronization | DCS-FR-CSA-06, UC-03-05 | Machine-readable payload and human-readable rendering/version | Same semantics/version before archival; inconsistencies highlighted | Missing field, reordered/changed clause, rendering omission, wrong version/tag | Marked Done/Ongoing across sources; evidence required |
| VAL-16 Pre-archive compliance | DCS-FR-CSA-07 | Finalized contract plus configured rules/regulations | Non-compliance is flagged for review or blocks storage | Missing rule pack, rule error, warning-vs-blocking boundary, dependency outage | Marked Done; outcome choice underspecified |
| VAL-17 Archived-contract compliance | DCS-FR-CSA-19 | Archived document, retention/signature/metadata policy | Non-compliant archive entry is flagged | Expired retention, invalid signature, incomplete metadata, immutable evidence unavailable | Marked Done; policy profiles absent |
| VAL-18 Workflow regulatory/policy validation | DCS-FR-PACM-03 | Contract at each gated workflow stage plus eIDAS/GDPR/ISO/internal rules | Failure blocks execution or is routed to manual review | Rule/version change, conflicting rules, unavailable evaluator, warning-vs-blocking boundary | Ongoing; exact gates and rule packs absent |
| VAL-19 Multi-contract structural integrity | DCS-FR-PACM-06 | Package with main contract, annexes and sub-agreements | Complete, logically correct linkage before execution | Missing/orphan/duplicate/cyclic/wrong-version component; cross-instance reference | Not Started |
| VAL-20 Template correctness/semantics/authenticity | UC-02-07 | Template, JSON-LD context, SHACL/schema, signature/VC provenance | Report lists schema and authenticity checks; failures block generation | Invalid JSON-LD/SHACL, missing context, signature/VC invalid, mixed pass/fail | Ongoing |
| VAL-21 Counterparty signature use case | UC-04-03 | PDF, expected document hash, VC/status response | Report shows PDF integrity, hash match and status retrieval | Full status-list validation must be limited/blocked, never reported passed, with incompatible bit order | Ongoing; adjusted external dependency |
| VAL-22 Pre-execution contract validation | UC-10-02 | Deployable contract plus integrity/compliance rules | Violations block deployment; detailed report stored with contract | Report persistence failure, stale rules, warning/block boundary, repeatability | Not Started |
| VAL-23 System-driven API review | UC-12-02 | Contract review API request and rule set | Issues returned; failing contract cannot proceed to approval | Empty/malformed request, unauthorized caller, evaluator failure, concurrent transition | Marked Done; evidence required |
| VAL-24 OIDC token validation | DCS-IR-SI-07, DCS-IR-CI-05 | Discovery metadata, JWKS and token/client assertions | Issuer, signature, audience/client, expiry and required claims validate | Key rotation, unknown `kid`, stale cache, bad issuer/audience/clock, discovery outage | Marked Done |
| VAL-25 Credential/status validation interface | DCS-IR-SI-09, DCS-IR-CI-09 | Credential/contract identifier and compatible status list | Current state used for enforcement; publication visible within <= 5 minutes | Encoding incompatibility, stale response, unknown entry, outage, suspended/revoked transition | Done/Ongoing; compatible encoding is material |
| VAL-26 DID/VC cryptographic verification interface | DCS-IR-SI-12 | DID document or VC/VP plus trust/resolution inputs | DID/VC/VP proof validates for wallet integration | Resolution failure, unsupported proof, revoked/rotated key, domain/challenge mismatch | Ongoing |
| VAL-27 C2PA compound verifier | Supplementary DCS-OR-C2PA-006 | PDF plus embedded/remote C2PA, VC and live status | Four independently named checks and correct lifecycle banner | One failed/unavailable check must not be masked by others | Applicability pending; traceability exists, not proof |
| VAL-28 C2PA/PDF update compatibility | Supplementary DCS-OR-C2PA-002/-010 | Legally signed PDF and one or more C2PA increments | Both signature families remain valid after every update | Update/sign ordering, repeat update, corrupted xref/increment, supported signature profiles | Applicability pending |
| VAL-29 C2PA remote fallback | Supplementary DCS-OR-C2PA-008 | PDF with stripped/missing embedded credential plus contract-bound remote link | Verifier obtains and validates remote manifest | Missing/wrong/tampered/unavailable remote artifact; link substitution | Applicability pending |
| VAL-30 C2PA VC/status binding | Supplementary DCS-OR-C2PA-004/-005 | Lifecycle assertion, status VC and published list | All binding fields agree and status publication meets <= 5 minutes | Field mismatch, revoked/untrusted VC, stale list, time boundary | Applicability pending |

## C. Validation surfaces that must not become separate truth sources

These UI/API requirements expose validation paths but do not define independent validation semantics.
They must delegate to the same authoritative rule implementations and return their real findings.

| Surface requirement | Must expose / delegate to |
|---|---|
| DCS-IR-TR-03/-04/-06 | VAL-01, VAL-02, VAL-03 and VAL-20; verified state gates approval |
| DCS-IR-CWE-05 | VAL-05 and, where applicable, VAL-15/VAL-18 |
| DCS-IR-SM-02/-04 | VAL-11 and VAL-12 |
| DCS-IR-SM-05/-07 | VAL-14 plus trust-anchor, cryptographic-proof and timestamp detail from VAL-12 |
| DCS-IR-SM-08 | Export the actual VAL-12/VAL-14 findings, including unavailable/limited checks |
| DCS-IR-CSA-04 | Archive-operation and integrity evidence from VAL-15 through VAL-17 |
| DCS-IR-PACM-01 through -04 | Initiate, monitor, report and link VAL-01/VAL-18/VAL-22 findings without redefining rules |

## D. Cross-cutting evidence rules for the later verification sweep

Each validation path needs all of the following before it can be classified as satisfied:

1. A stable trigger or API/UI entry point and an identified authorization rule.
2. A named, versioned source for schemas, policies, trust anchors, algorithms, status formats and
   lifecycle mappings; no component-local hard-coded substitute.
3. Independent results for every check. `unavailable`, `limited`, `blocked` and `not_applicable`
   must not be collapsed into `passed`.
4. A deterministic positive fixture and at least one negative fixture per listed failure class.
5. Boundary cases for time, lifecycle state, concurrency, stale caches/snapshots and external
   dependency failure where applicable.
6. Enforcement evidence: a finding is insufficient where the requirement says block, reject,
   invalidate, preserve persistence, or prevent archival/deployment/approval.
7. Durable, correlated evidence containing requirement/path ID, input identity/version, rule-set
   version, timestamp, actor, findings and outcome, without leaking secrets or excess personal data.
8. An independently repeatable command or scenario. Comments, tags and status fields are only
   traceability hints.

## E. Confirmed requirement defects and unresolved decisions

- `DCS-FR-SM-03.effective_requirement` is truncated at `The syste`; the authoritative source must
  be repaired before this path can be considered fully specified.
- All 32 NFR measurement vectors lack complete targets/test profiles. They cannot currently yield
  an objective pass/fail result without an approved interpretation.
- C2PA-001 through -010 have measurable targets but are absent from the primary interpretation.
  Their applicability is therefore **pending**, not implicitly mandatory and not obsolete.
- Domain-specific regulatory rule packs for template and contract compliance are not fully named or
  versioned. Generic references to eIDAS/GDPR/ISO are not executable validation oracles.
- Several effective requirements permit `flag for review` **or** `block`. The decision boundary must
  be explicit per rule severity and lifecycle gate.
- UC-04-03 explicitly limits full status-list validation because of an incompatible external
  encoding. Any report that calls retrieval alone a successful status validation is a false positive.
- `Done` and `Ongoing` values conflict for some linked paths (notably MR/HR consistency). The later
  evidence sweep must assess each atomic obligation instead of inheriting an aggregate status.

## F. Next sweep phase

For every row above, locate candidate implementation, unit/integration/BDD evidence, configuration and
external dependency. Then execute or inspect that evidence against each atomic positive, negative and
boundary case. The result will add per-obligation classifications: `verified`, `partial`, `missing`,
`blocked_external`, `underspecified`, or `not_applicable_by_decision` with direct evidence links.

### F.1 Initial trace observations

- Exact feature tags currently point to candidate scenarios for VAL-01/02/03/07/08/10/12/13/14/15,
  VAL-18/19/20/21/23 and several C2PA paths. Those tags have not yet been accepted as evidence.
- The PoA credential scenarios tagged for DCS-FR-SM-03 and DCS-FR-SM-04 are skipped.
- A certificate-revocation scenario under signature validation is skipped, so the presence of other
  DCS-FR-SM-18 scenarios cannot establish complete credential-status coverage.
- C2PA feature tags were found for C2PA-001, -002, -003, -006, -007, -008 and -010. No exact feature
  tag was found for C2PA-004, -005 or -009 in the initial pass.
- The C2PA conformance feature itself contains skipped cases for stripped-metadata verification and
  the `replaced` banner. Those two target dimensions are therefore not presently evidenced by that
  feature even if its non-skipped scenarios pass.
- Direct NFR feature tags are sparse relative to the 32 measurement vectors and cannot compensate
  for the missing measurement targets identified in section A.
