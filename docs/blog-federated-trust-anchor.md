# Vertrauen in einer Föderation: Self-signed ToS-Credentials, Policy-Endpunkte und was Gaia-X wirklich verlangt

*Ein Forschungs-Run-down zur Frage, wie zwei souveräne Instanzen einander vertrauen können — anhand des Gaia-X Trust Framework, der Eclipse Cross Federation Services Components (XFSC), des Eclipse-Dataspace-Components-Trust-Modells (EDC/DCP), des W3C Verifiable Credentials Data Model 2.0 und der eIDAS-2.0-Verordnung (EU) 2024/1183. Alle Zitate aus Verordnungen und Spezifikationen sind verbatim aus den Primärquellen gezogen; sämtliche Kernaussagen wurden in einer zweiten Runde adversarial gegengeprüft (Stand: Juli 2026).*

---

## 1. Das Problem

Ein Digital Contracting Service (DCS) verwaltet verhandelbare digitale Verträge, die unter eIDAS mit fortgeschrittenen oder qualifizierten elektronischen Signaturen (AES/QES) unterzeichnet werden. Zwei Instanzen — je eine pro Vertragspartei — tauschen Angebote, Gegenangebote und signierte Dokumente direkt aus, ohne zentralen Vermittler. Damit stellen sich vor **jeder** Interaktion zwei getrennte Fragen:

1. **Identität:** Ist mein Gegenüber, wer es vorgibt zu sein?
2. **Regelakzeptanz:** Hat sich mein Gegenüber den Spielregeln der Föderation unterworfen?

Unsere Design-Idee: Jeder Participant veröffentlicht ein **self-signed ToS-Credential** (ein W3C Verifiable Credential) unter seiner eigenen Instanz, sodass unabhängige Dritte prüfen können, ob er mit den Regeln der Software einverstanden ist. Zusätzlich exponiert jede Instanz einen **Low-Code-Policy-Endpunkt**, der zunächst nur `200 OK` liefert, aber als Extension-Punkt für dataspace-abhängige Policies dient — bis hin zur Anfrage an ein Gaia-X Clearing House. Jede Cross-Instance-Interaktion muss das Credential prüfen und den Policy-Endpunkt konsultieren.

Die Forschungsfrage: Ist das ein legitimes Trust-Muster? Passt es in die XFSC-Vision? Und wie verhält es sich zu eIDAS 2.0?

**Kurzantwort: Ja, ja — mit einer entscheidenden Präzisierung: Das self-signed Credential ist der *Einstieg* in die Vertrauenskette, nicht ihr *Anker*. Der Anker ist das, was der Policy-Endpunkt befragt.**

---

## 2. Was Gaia-X wirklich sagt: Self-signed ist Pflicht — aber nie genug

### 2.1 Das self-signed ToS-Credential ist ein Gaia-X-Kernbaustein

Die Sorge, ein selbst signiertes Credential sei ein Design-Hack, ist unbegründet. Gaia-X selbst macht genau das zur Pflicht. Das Trust Framework 22.10 verlangt:

> "Each issuer shall issue a `GaiaXTermsAndCondition` verifiable credential"

— mit Trust Anchor **`issuer`** (also self-signed) und dem "SHA512 of the Generic Terms and Conditions for Gaia-X Ecosystem" als Inhalt. (Einordnung: 22.10 ist eine historische Fassung — der „Trust Framework" heißt heute **Gaia-X Compliance Document**, aktuelle Version Loire/25.10. Das Self-signed-Muster besteht dort fort, nur in anderer Kredentialform: das *"Issuer (Terms and Conditions) Credential"* muss weiterhin *"be prepared and to be signed by the participant"*, inzwischen als VC-JOSE enveloped credential.) Das Compliance Document 25.10 (Kriterium PA1.1) formuliert:

> "Issuance of a Gaia-X Participant and a Gaia-X Terms & Conditions credential according to the Gaia-X Ontology, signed with the same cryptographic material issued by an applicable Trust Anchor."

Und der offizielle Conformance-Leitfaden beschreibt den Workflow so:

> "User signs their credentials with their private key... the user is free to choose their preferred signing tool."

Das vorgeschlagene Muster — jeder Participant publiziert unter seiner Instanz ein selbst signiertes ToS-VC — spiegelt also exakt das Gaia-X-eigene Design.

### 2.2 Die Grenze: Self-signed ist Einstieg, nicht Anker

Ebenso eindeutig ist die Gegenrichtung. Ein self-signed Credential allein stiftet in Gaia-X **keine** Identität und **keine** Konformität:

- Ein Participant muss eine identifizierte, onboarded Rechtsperson sein: *"A Participant is a Legal Person or Natural Person, which is identified, onboarded and has a Gaia-X Self-Description. Instances of a Participant neither being a legal nor a natural person are prohibited."*
- Identitätsattribute sind drittverankert: `registrationNumber` gegen einen `registrationNumberIssuer` (Notar), `countryCode` gegen den Staat.
- Konformität entsteht erst durch die Gegenzeichnung des Clearing House: Das Gaia-X Digital Clearing House (GXDCH) *"validates the shape and content of the Gaia-X Credentials and, if successful, issues a GaiaXComplianceCredential"* — erst dieses vom Clearing House signierte Credential berechtigt zur Konformitätsbehauptung.

Das Vertrauensmodell ist also ein **Hybrid**: selbst ausgestellte Erklärungen (ToS, Self-Description), gegengezeichnet durch eine Compliance-Attestierung, wobei der Signaturschlüssel selbst an akzeptierte Trust Anchors kettet.

### 2.3 Accountability: Warum die Selbsterklärung trotzdem Zähne hat

Die Selbsterklärung ist in einen Haftungsmechanismus eingebettet, den ein Policy-Endpunkt operationalisieren kann:

> "The certificate or public key of the keypair used to sign Gaia-X Credentials will be marked as untrusted where the Gaia-X European Association for Data and Cloud becomes aware of any inaccurate statements..."

Die Gaia-X Registry *"stores trust anchors accepted by the Gaia-X Trust Framework"*, und alle GXDCH-Dienste (Registry, Compliance, Notary) sind als öffentliche, versionierte Web-APIs verfügbar — im Fact-Check live verifiziert (`https://registry.gaia-x.eu/v1/docs/` und `https://compliance.gaia-x.eu/v1/docs/` liefern beide die Swagger-UI) — direkt aufrufbar, auch gegen eine konkret gewählte Clearing-House-Instanz: *"A direct API call is also possible... Calling directly the API of the clearing house."* Der Pfad „erweiterbarer Policy-Endpunkt, der später ein Clearing House befragt" ist damit architektonisch vorgesehen, nicht spekulativ.

### 2.4 Identitätsbindung: eIDAS-Zertifikat hinter did:web

Wie beweist ein Gegenüber, dass die Rechtsperson hinter seiner DID real ist?

> "User has an EV SSL or an eIDAS certificate and the public part of the certificate is published via DID:WEB method."

Das Gaia-X ICAM-Dokument verlangt, dass der `publicKeyJwk` des Issuers eine `x5c`/`x5u`-Kette trägt, die zu einem gültigen Gaia-X Trust Anchor auflöst. **Von dieser Kette erbt das self-signed ToS-Credential seine Identitätssicherung** — nicht umgekehrt.

---

## 3. XFSC: Der Policy-Endpunkt existiert bereits als Spezifikation

### 3.1 TSA — die Blaupause des Low-Code-Endpunkts

Die XFSC Trust Services API (TSA) spezifiziert Policy-Evaluierung als *"provisioning of versioned HTTP routes to execute the policy"*. Die Referenzimplementierung exponiert `GET/POST /policy/{repository}/{group}/{policy}/{version}/evaluation`, ausgewertet durch OPA/Rego. Entscheidend ist die normative Erweiterbarkeit ([IDM.TSA.E1.00013]):

> "The decision engine MUST have the capability to call HTTP URLs with free chosen query parameters, headers, and request bodies for any HTTP verb within the policy execution."

Die eingebauten Rego-Extension-Funktionen umfassen `proof.verify` (*"Verify a proof for Verifiable Credential or Verifiable Presentation"*), `did.resolve` (*"Resolve DID using the Universal DID Resolver"*), `external.http.header()`, `cache.get/set` und `task.create`. Eine triviale Allow-All-Policy (`200 OK`) kann also **in place** erweitert werden: erst ToS-VC-Proof prüfen, dann DID-Schlüssel auflösen, dann GXDCH befragen. Genau das ist unser Muster — XFSC hat es bereits standardisiert.

Das erklärte Ziel der TSA deckt sich mit unserem: *"The aim of the Trust Services API Extension 1 is to ensure a consistent level of trust between Gaia-X participants and components."*

### 3.2 TRAIN — warum das Credential nicht der Anker sein darf

Die XFSC-Komponente TRAIN (Trust Management Infrastructure) existiert, um Issuer-Vertrauen zu **externalisieren**:

> "...establishing and verifying the root of trust for participants in the distributed Gaia-X ecosystem... through the introduction of trust lists combined with anchoring of pointers in the DNS."

> "Trust Frameworks are anchored in DNS Pointer Resource Record (PTR RR)... Trust List URI DID is anchored in DNS URI Resource Record (URI RR)."

Und normativ: Die Credential-Verifikation MUSS den Issuer gegen eine Föderations-Trust-List prüfen — zusätzlich zur kryptografischen Integrität: *"Verification of issuer details of the credential with the information of the trust list... Integrity of VC must also be verified."* Das Enrollment neuer Issuer ist ein expliziter Schritt: *"The enrollment process of new issuers is a process to set a DID and the respective configuration on a trustlist of a federation... the TRAIN enrollment module needs to be called."*

Die XFSC Notarization API verzahnt sogar `termsOfUse` direkt mit TRAIN ([CP.NOTAR.E1.00015]): *"The TRAIN validation service needs to be included in the process of verifying a verifiable presentation... the notarization service needs to validate the terms Of Use by calling the TRAIN validation service."*

**Fazit XFSC-Fit:** Die Idee passt präzise in die XFSC-Vision — unter einer Bedingung: Der Policy-Endpunkt ist ein *Policy Decision Point*, kein Trust Anchor. Der Anker ist die Trust-List / das Clearing House, das die Policy befragt. Die `200 OK`-Stufe ist als Bootstrap legitim; zum Trust Anchor wird der Endpunkt erst durch seinen Extension-Pfad.

### 3.3 eIDAS-Haken in XFSC

XFSC nennt eIDAS-Signaturen explizit als Pflichtbestandteil ([IDM.TSA.E1.00021]):

> "Signatures must be generated/verified in compliance with eIDAS so that legally secure trust can be achieved. This should include the eIDAS signature types basic, advanced, and qualified."

Ein Föderations-Trust-Mechanismus mit AES/QES-Vertragssignatur liegt damit im Kern der XFSC-Vision, nicht an ihrem Rand.

---

## 4. W3C Verifiable Credentials 2.0: `termsOfUse` ist genau dafür da

Das VC Data Model 2.0 (W3C Recommendation vom 15. Mai 2025, Abschnitt 5.5 "Terms of Use") definiert verbatim — die Property stand während der Standardisierung zur Debatte („reserved property"), blieb in der finalen Recommendation aber normativ definiert:

> "Terms of use can be used by an issuer or a holder to communicate the terms under which a verifiable credential or verifiable presentation was issued. The issuer places their terms of use inside the verifiable credential."

> "The value of the `termsOfUse` property MUST specify one or more terms of use policies under which the creator issued the credential or presentation. If the recipient (a holder or verifier) is not willing to adhere to the specified terms of use, then they do so on their own responsibility and might incur legal liability if they violate the stated terms of use. Each `termsOfUse` value MUST specify its `type`, for example, `TrustFrameworkPolicy`, and MAY specify its instance `id`."

Das Beispiel der Spec nutzt exakt das Föderationsmuster:

```json
"termsOfUse": {
  "type": "TrustFrameworkPolicy",
  "trustFramework": "Employment&Life",
  "policyId": "https://policy.example/policies/125",
  "legalBasis": "professional qualifications directive"
}
```

Zwei Design-Konsequenzen:

1. **Modellierung:** Ein „ToS-Credential" ist spec-konform am saubersten als Participant-Credential *mit* `termsOfUse`-Property (Typ z. B. `TrustFrameworkPolicy`, `policyId` + Hash der Regeln) — nicht als freischwebendes Credential, dessen einziger Inhalt die ToS sind. Gaia-X' `GaiaXTermsAndConditions`-VC zeigt, dass auch die dedizierte Variante gangbar ist; beide referenzieren die Regeln per URL + Hash.
2. **Vertrauen bleibt Verifier-Sache:** Die Spec stellt klar: *"Verifiability of a credential does not imply the truth of claims encoded therein... a verifier validates the included claims using their own business rules before relying on them."* Das Datenmodell stiftet kein Issuer-Vertrauen — genau die Lücke, die Trust-Lists (TRAIN), Clearing Houses (GXDCH) oder eben unser Policy-Endpunkt füllen müssen.

---

## 5. eIDAS 2.0: Die Rechtsebene

Die Verordnung (EU) 2024/1183 ändert die Verordnung (EU) 910/2014 in place; die Signatur-Grundpfeiler bleiben bestehen (verbatim aus 910/2014):

**Artikel 25 — Legal effects of electronic signatures:**

> "1. An electronic signature shall not be denied legal effect and admissibility as evidence in legal proceedings solely on the grounds that it is in an electronic form or that it does not meet the requirements for qualified electronic signatures.
> 2. A qualified electronic signature shall have the equivalent legal effect of a handwritten signature."

**Artikel 26 — Requirements for advanced electronic signatures:**

> "An advanced electronic signature shall meet the following requirements: (a) it is uniquely linked to the signatory; (b) it is capable of identifying the signatory; (c) it is created using electronic signature creation data that the signatory can, with a high level of confidence, use under his sole control; and (d) it is linked to the data signed therewith in such a way that any subsequent change in the data is detectable."

Neu in 2024/1183 sind die **elektronischen Attributsbescheinigungen** (Art. 3, neue Punkte):

> "(44) 'electronic attestation of attributes' means an attestation in electronic form that allows attributes to be authenticated;
> (45) 'qualified electronic attestation of attributes' means an electronic attestation of attributes which is issued by a qualified trust service provider and meets the requirements laid down in Annex V;"

Mit eigener Rechtswirkung (neuer **Artikel 45b** — Legal effects of electronic attestation of attributes):

> "1. An electronic attestation of attributes shall not be denied legal effect or admissibility as evidence in legal proceedings on the sole ground that it is in electronic form or that it does not meet the requirements for qualified electronic attestations of attributes.
> 2. A qualified electronic attestation of attributes and attestations of attributes issued by, or on behalf of, a public sector body responsible for an authentic source shall have the same legal effect as lawfully issued attestations in paper form."

Dazu das European Digital Identity Wallet (Art. 3 Punkt 42):

> "'European Digital Identity Wallet' means an electronic identification means which allows the user to securely store, manage and validate person identification data and electronic attestations of attributes for the purpose of providing them to relying parties and other users of European Digital Identity Wallets, and to sign by means of qualified electronic signatures or to seal by means of qualified electronic seals;"

— mit der politischen Ansage (Erwägungsgrund 20): *"The use of a qualified electronic signature should be free of charge to all natural persons for non-professional purposes."*

**Was heißt das für unser Muster?**

- **Die Identitätsfrage („ist er, wer er vorgibt zu sein?") beantwortet eIDAS, nicht das ToS-VC.** Die Bindung Person↔Signatur leistet die AES/QES auf dem Vertrag (Art. 26 (a)–(b): *uniquely linked*, *capable of identifying the signatory*); die Bindung Instanz↔Rechtsperson leistet die eIDAS-Zertifikatskette hinter dem `did:web`-Dokument.
- **Art. 45b Abs. 1 gibt sogar dem nicht-qualifizierten ToS-VC einen rechtlichen Boden:** Es darf als Beweismittel nicht allein deshalb abgelehnt werden, weil es elektronisch und nicht qualifiziert ist. Ein self-signed ToS-VC ist damit eine rechtlich verwertbare Willensbekundung („ich akzeptiere die Föderationsregeln") — nur eben keine drittbestätigte.
- **Der Upgrade-Pfad ist angelegt — beendet aber das Self-signed-Modell:** Wird das ToS-/Participant-Credential eines Tages als **QEAA** ausgestellt, erhält es per Art. 45b Abs. 2 die Wirkung einer papierförmigen Bescheinigung. Annex V macht dabei unmissverständlich, dass ein QEAA per Definition nicht mehr self-signed sein kann — es muss u. a. enthalten: *"(g) the qualified electronic signature or qualified electronic seal of the issuing qualified trust service provider"* und *"(i) the information or location of the services that can be used to enquire about the validity status of the qualified attestation"*. Der QEAA-Pfad ist also kein Upgrade *des* self-signed Credentials, sondern seine Ablösung durch eine drittattestierte Föderationsmitgliedschaft mit eigenem Revocation-Dienst.

---

## 6. Gegenprobe: Wie es die Dataspace-Welt macht (EDC/DCP/Catena-X)

Die härteste Gegenprobe liefert das Trust-Modell der Eclipse Dataspace Components (EDC) mit dem Decentralized Claims Protocol (DCP, vormals IATP). Das Dataspace Protocol (DSP) selbst definiert bewusst kein Trust-Modell — *"The Dataspace Protocol Specifications do not address how participant identities are determined or trust is verified, instead leaving it to individual dataspaces"* — DCP füllt diese Lücke. Der Vergleich bestätigt unser Muster in zwei Punkten und korrigiert es in zwei anderen:

**Bestätigt — dezentrales Credential-Hosting beim Participant:** Im DCP-Modell hält jeder Participant seine Credentials selbst (IdentityHub / Credential Service), auffindbar über einen `service`-Eintrag im eigenen `did:web`-Dokument; der Verifier holt sich die Verifiable Presentation direkt dort ab. Genau unser „unter seiner Instanz veröffentlicht"-Muster.

**Bestätigt — Pflicht-Check bei jeder Interaktion:** Das EDC Minimum Viable Dataspace formuliert wörtlich: *"it is a dataspace rule that the `MembershipCredential` must be presented in every DSP request"*. Tractus-X/Catena-X prüft entsprechend in jeder Access Policy den Constraint `{"leftOperand": "Membership", "operator": "eq", "rightOperand": "active"}`. Ein Regelakzeptanz-Nachweis vor jeder Cross-Instance-Interaktion ist dort gelebte Praxis.

**Korrigiert — self-signed stiftet dort keinen Trust:** Im EDC/Catena-X-Modell ist das trust-stiftende Credential (Membership, Framework Agreement) **immer von einem dataspace-weiten Issuer ausgestellt** — der Governance Authority bzw. deren Trust-Anchor-Betreiber (*"The Association defines the Issuer of verifiable credentials"*, CX-0006). Der Verifier prüft gegen eine konfigurierte Issuer-Allowlist (`edc.iam.trusted-issuer.*`); ein Credential eines ungelisteten Issuers wird verworfen. Self-signed ist im DCP nur der Self-Issued ID Token — reiner Identitäts-/Key-Possession-Nachweis (*"The `iss` and `sub` claims MUST be equal and set to the bearer's (participant's) DID"*), keine Claim-Attestierung. Das deckt sich exakt mit dem Gaia-X/TRAIN-Befund: Self-signed ist Einstieg, der Anker ist extern.

**Korrigiert — der Policy-Check ist lokal, kein Callback auf die Gegenseite:** EDC kennt keinen öffentlichen Policy-Endpunkt, den der *Gegenüber* vor jeder Interaktion aufruft. Der Check ist eine **lokale, erweiterbare Policy Engine des Verifiers** (registrierbare Policy Functions, die auch Backend-Lookups machen können), eingebettet in Catalog/Negotiation/Transfer. DCP vermeidet Online-Clearing pro Interaktion bewusst — es *"avoids the need for third-party verification"* und begrenzt *"third parties' ability to eavesdrop"*. Dieselbe Sorge formuliert die W3C-VC-Spec bei `termsOfUse` („to avoid 'phone home' privacy issues").

**Design-Konsequenz für unser Muster:** Der Low-Code-Policy-Endpunkt gehört als **lokaler Policy Decision Point des Verifiers** geschnitten — jede Instanz befragt *ihren eigenen* Endpunkt, bevor sie eine eingehende oder ausgehende Interaktion zulässt. Er ist Deployment-Detail der eigenen Instanz (deshalb Low-Code: der Betreiber passt ihn an seinen Dataspace an), kein öffentlich konsultierbarer Dienst der Gegenseite. So bleibt die Privacy-Eigenschaft von DCP erhalten, und der Extension-Pfad (Trust-List, GXDCH) läuft als Backend-Lookup des eigenen PDP statt als „phone home" zwischen den Parteien.

---

## 7. Abgleich mit der DCS-Codebase: fast alles ist schon da

Die überraschendste Erkenntnis der Untersuchung: Das DCS implementiert das Schichtenmodell bereits in weiten Teilen.

**Vorhandene Trust-Layer zwischen Instanzen** (`backend/internal/base/identity/did.go`, `backend/internal/dcstodcs/`):

1. **eIDAS-Zertifikatskette im `did:web`-Dokument**, geprüft gegen den EU-Trust-Pool (`VerifyEIDASCertificate`) — exakt das Gaia-X-Muster „eIDAS-Zertifikat via did:web publiziert".
2. **HSM-signierte Challenge-Response pro Request** — der Sender signiert mit seinem PKCS#11-Schlüssel, der Empfänger löst die `did.json` des Senders auf und verifiziert. Kein mTLS, keine API-Keys: Vertrauen liegt in der DID, nicht im Transport.
3. **Lokale `trusted_peers`-Allowlist** (`trustedpeercheck.go`, im Code explizit als "third trust layer" bezeichnet) — auch ein kryptografisch valider Peer muss gelistet sein.

**Bausteine für das neue Muster:**

- Das Gaia-X-Participant-Beispiel `docs/fc-integration/examples/participant_sd.jsonld` ist bereits ein W3C-VC mit `gx-service-offering:TermsAndConditions {url, hash}` — das ToS-VC existiert prototypisch.
- VC/VP-Verifikation (SD-JWT, Status Lists, Issuer-Trust-Config) läuft produktiv im OID4VP-Login (`auth/oid4vp/`).
- `deployment/node-red/` liegt im Stack — die Low-Code-Runtime für den `200 OK`-Bootstrap-Endpunkt ist schon deployt.
- Der Gaia-X-Federated-Catalogue-Client (`internal/templatecatalogueintegration/`) liefert die Anbindung, über die der Policy-Endpunkt später ein Clearing House befragen kann.
- Signatur-Stack: JAdES für die maschinenlesbare Ebene, PAdES-B-T mit RFC3161-Timestamp, DSS-Validierung nach ETSI EN 319 102-1, `AssertValidAES` gegen eIDAS Art. 26.

**Der Einhängepunkt** ist eindeutig: Die dritte Trust-Schicht (`CheckForUntrustedPeers`) wird von der statischen Allowlist zum Policy-Aufruf — empfangsseitig in `PostPdf` nach eIDAS-Kette und Challenge-Verifikation, sendeseitig vor `shipToPeers`. Die ersten beiden Schichten bleiben unangetastet.

**Ein bewusst stehenzulassender „Reibungspunkt":** Die Instanz-Identität ist im DCS hart an eIDAS-qualifizierte Zertifikate und Hostname-Matching gebunden. Das ist kein Hindernis, sondern die korrekte Arbeitsteilung — das self-signed ToS-VC ersetzt die Identitätsprüfung nicht, es ergänzt sie um die Regelakzeptanz-Dimension.

---

## 8. Der verbindliche Run-down: Vier Schichten föderierten Vertrauens

Aus Standards, Verordnung und Codebase ergibt sich ein Schichtenmodell, das nicht nur für das DCS, sondern für jedes föderierte System trägt:

| Schicht | Frage | Mechanismus | Anker |
|---|---|---|---|
| **1. Identität** | Wer bist du? | `did:web` + eIDAS-Zertifikatskette (`x5c`) im DID-Dokument, Challenge-Response pro Request | EU Trusted Lists / QTSP |
| **2. Regelakzeptanz** | Spielst du nach den Regeln? | Self-signed ToS-VC (`termsOfUse`, Typ `TrustFrameworkPolicy`, Regeln als URL + Hash), publiziert unter der eigenen Instanz | Schicht 1 (Signaturschlüssel) + Accountability der Föderation |
| **3. Policy** | Darf *diese* Interaktion stattfinden? | **Lokaler** Low-Code Policy Decision Point je Instanz (HTTP, OPA/Rego-fähig); Bootstrap: `200 OK`; Extension: Trust-List-Check, GXDCH-Compliance-Abfrage, Revocation — als Backend-Lookup des eigenen PDP, kein Callback auf die Gegenseite | Trust-List / Clearing House (das, was der Endpunkt befragt) |
| **4. Rechtsverbindlichkeit** | Gilt der Vertrag? | AES/QES auf dem Dokument (PAdES/JAdES), DSS-validiert | eIDAS Art. 25/26; perspektivisch EUDI Wallet + QEAA |

Vier Merksätze:

1. **Self-signed Credentials sind legitim und sogar vorgeschrieben — als Einstieg.** Gaia-X verlangt das self-signed `GaiaXTermsAndConditions`-VC von jedem Issuer. Aber Identität kommt aus Schicht 1, Konformität aus der Gegenzeichnung. Wer das self-signed Credential selbst zum Anker erklärt, baut ein Tautologie-Vertrauen („ich vertraue dir, weil du sagst, dass man dir vertrauen kann").
2. **Der Policy-Endpunkt ist Policy Decision Point, nicht Trust Anchor.** XFSC/TSA hat exakt diese Form standardisiert — versionierte HTTP-Routen, normativ verpflichtende externe HTTP-Calls, `proof.verify` und `did.resolve` als eingebaute Funktionen. Die `200 OK`-Stufe ist ein ehrlicher Bootstrap; zum Trust-Mechanismus wird der Endpunkt durch das, was er befragt.
3. **eIDAS trägt die Rechtsebene, die VC-Welt die Semantikebene — sie treffen sich im DID-Dokument.** Die AES/QES beweist, *wer unterschrieben hat*; die eIDAS-Kette hinter `did:web` beweist, *wem die Instanz gehört*; das ToS-VC beweist, *worauf man sich eingelassen hat* — und ist dank Art. 45b auch unqualifiziert rechtlich verwertbar.
4. **Der PDP ist lokal — kein „phone home" zwischen den Parteien.** EDC/DCP und die W3C-VC-Spec meiden aus Privacy-Gründen Online-Checks bei Dritten pro Interaktion. Jede Instanz befragt ihren *eigenen* Policy-Endpunkt; externe Anker (Trust-List, GXDCH) werden als Backend-Lookup dieses PDP angebunden, nicht als öffentlicher Dienst, den die Gegenseite aufrufen muss.

**Für die Fragezeichen im Projekt heißt das:** Sie kippen. Das Muster ist kein Sonderweg, sondern die Konvergenz dessen, was Gaia-X vorschreibt, XFSC spezifiziert und eIDAS 2.0 rechtlich unterfüttert. Die DCS-Architektur (drei Trust-Layer, did:web + HSM, VC-Verifikation, Node-RED, FC-Client) muss dafür nicht umgebaut, sondern nur an einer Stelle — der dritten Trust-Schicht — geöffnet werden.

---

## 9. Grenzen und offene Punkte

Redliche Forschung nennt ihre Ränder:

- **Die `200 OK`-Stufe ist nur als Bootstrap verteidigbar.** Ohne den Extension-Pfad (Trust-List oder GXDCH) prüft sie nichts und verankert nichts. Sie gehört mit einem klaren Ausbaupfad dokumentiert, sonst wird aus dem Extension-Punkt ein Feigenblatt.
- **XFSC lebt weiter — aber selektiv.** Die alten Eclipse-GitLab-Repos sind archiviert; der Code liegt heute unter `github.com/eclipse-xfsc` (TSA-Policy-Service als `custom-policy-agent` — Go, Goa v3 + OPA, derselbe Stack wie das DCS-Backend). Aktiv gepflegt werden 2026 aber vor allem Federated Catalogue, der OID4VCI/SD-JWT/Statuslist-Stack, die Crypto-Provider und die `facis-*`-Komponenten (IPCEI-CIS/8ra-Förderung) — TSA, TRAIN und Notarization erhalten seit Mitte 2025 nur noch CI-Pflege, ohne Releases. Konsequenz: TSA/TRAIN als *Spezifikations-Referenz* nutzen, den PDP selbst betreiben (OPA hinter Node-RED, beides bereits im DCS-Stack). Bemerkenswert: Das DCS selbst ist Teil dieser aktiven FACIS-Linie — das hier beschriebene Muster entsteht also innerhalb der XFSC-Nachfolge, nicht neben ihr.
- **Gaia-X versioniert aggressiv.** Trust Framework 22.10 → Compliance Document Tagus/Loire (aktuell 25.10) — das Self-signed-ToS-Muster ist über die Versionen stabil, aber Credential-Namen, Formate (VC-JOSE enveloped) und Kriteriennummern verschieben sich. Integrationen sollten gegen die versionierten GXDCH-APIs gebaut werden, nicht gegen Dokumentstände.
- **EDC/DSP wurde in der zweiten Runde abgedeckt** (Abschnitt 6): Es bestätigt dezentrales Credential-Hosting und den Pflicht-Check pro Interaktion, widerspricht aber der Self-signed-Attestierung als Trust-Quelle und dem öffentlich konsultierbaren Policy-Endpunkt — beides ist im Run-down eingearbeitet.
- **Der QEAA-Pfad ist geklärt, aber nicht beschritten:** Annex V (verbatim in Abschnitt 5) definiert die Anforderungen; welcher QTSP Föderationsmitgliedschaften als (Q)EAA ausstellen würde und zu welchen Konditionen, ist eine Markt-, keine Architekturfrage.

---

## Anhang: Minimaler Implementierungsvektor für das DCS

Wie andere XFSC-Komponenten das Dev-vs-Produktion-Problem lösen, folgt vier wiederkehrenden Mustern: **(1)** Boolean-Flags pro Prüfschritt statt globalem „dev mode" (Federated Catalogue: `verification.vp-signature`/`vc-signature` einzeln schaltbar), **(2)** Trust-Anchor als URL-Konfiguration (custom-policy-agent: `DID_RESOLVER_ADDR`; TRAIN-TCR: `tcr.dns.hosts` + eigener DNSSEC-`rootPath`), **(3)** Signer als Plugin (Vault/HSM-Provider in Prod, `crypto-provider-local-plugin`/`dummycontentsigner` in Dev), **(4)** Trust-Bootstrap per Seed bei *eingeschalteter* Verifikation — die Wurzel ist lokal, nicht der Check aus (EDC MVD deployt einen echten lokalen IssuerService im KinD-Cluster; TRAIN signiert die Zone mit eigenem NSD lokal; gx-compliance läuft als Docker-Image mit self-signed Cert und `production=false`, öffentlich als `compliance.lab.gaia-x.eu/v1-staging` mit „relaxed rules"). Das DCS folgt Muster 4 bereits: `DCS_TRUSTED_PEERS` seedet die Allowlist, `DCS_FORCE_EIDAS_CERT=false` lockert nur die QC-Statement-Pflicht, `OID4VP_TRUST_DATA_PATH` ist file-basierte Trust-Config pro Umgebung.

Daraus der Vektor — vier Schritte, jeder einzeln shippable, CI bricht zu keinem Zeitpunkt:

1. **Agreement-Credential publizieren** (nur Lesen, kein Gate): Goa-Endpunkt `/.well-known/dcs-agreement-credential.json` neben der `did.json`. Inhalt: VC nach dem Muster der Participant-Self-Description mit `termsOfUse {type: TrustFrameworkPolicy, policyId, hash}`, Issuer = eigene `ISSUER_DID`, signiert über den vorhandenen HSM-Key (`DCS_HSM_KEY_VC`). Die Föderationsregeln selbst liegen per **`go:embed`** im Binary; der referenzierte Hash wird beim Start daraus gerechnet. Damit ist die Regelakzeptanz an die Software-Version gebunden — kein konfigurierbarer Drift, zwei Instanzen derselben Version einigen sich automatisch, manipulierte Regeln fallen im Hash-Vergleich auf.

   Exemplarische Regel im eingebetteten Regeldokument:

   > *"The federator of the DCS agrees that users operating the system that are designated signatories are legally allowed to represent the operating party."*

   Diese Regel schließt gezielt die Lücke, die weder eIDAS noch die DID-Schicht abdeckt: Die AES/QES beweist, *wer* signiert hat (Art. 26: „uniquely linked to the signatory"), die Zertifikatskette hinter `did:web` beweist, *wem die Instanz gehört* — aber keine der beiden Schichten beweist die **Vertretungsmacht** der signierenden Person. Mit dem Agreement-Credential sichert der Federator sie zu; aus einer Einzelfallprüfung pro Signatur wird eine haftungsbewehrte Selbsterklärung auf Föderationsebene — dasselbe Accountability-Muster, mit dem Gaia-X self-signed Credentials Zähne gibt (Schlüssel wird bei „inaccurate statements" als untrusted markiert, Abschnitt 2.3).
2. **Agreement-Prüfung beim Peer-Kontakt** (AND zur bestehenden dritten Trust-Schicht): empfangsseitig in der `PostPdf`-Sequenz nach eIDAS-Kette und Challenge-Check, sendeseitig neben `CheckForUntrustedPeers` — `dcs-agreement-credential.json` des Peers holen, Signatur gegen den Key aus dessen `did.json` verifizieren (Code existiert in `base/identity`), Hash gegen den eigenen (embedded) Erwartungswert vergleichen. Verifikation bleibt auch in Dev an; die Wurzel ist die eingebettete Regeldatei.
3. **Lokaler PDP-Aufruf, opt-in per URL**: Wrapper um die dritte Trust-Schicht — ist `DCS_TRUST_PDP_URL` gesetzt, POST an den *eigenen* PDP mit `{peerDID, agreementCredential, direction, contractDID, targetState}`; 2xx = erlaubt, sonst Ablehnung mit Audit-Event (Senke: der `IncidentReport`-Flow). **Ungesetzt = exakt heutiges Verhalten** — das ist die CI-Erhaltungsgarantie. Der PDP ist der eigene, lokale Endpunkt; die Gegenseite ruft ihn nie auf (kein „phone home", vgl. Abschnitt 6).
4. **Node-RED als Default-PDP im Deployment**: ein Flow, der `200 OK` liefert, plus ein dokumentierter Beispiel-Flow gegen `compliance.lab.gaia-x.eu/v1-staging` (akzeptiert Nicht-EV-Zertifikate, also auch Dev-Identitäten). Für CI, die den PDP-Pfad testet: gx-compliance lokal als Container (analog zum SoftHSM2-Muster der HSM-Schicht) oder ein simpler HTTP-Stub im BDD-Setup (`features/17_peer_trust/`: Peer ohne/mit falschem Agreement-Credential wird abgelehnt → `sync_fails` + Incident im Audit-Trail).

Kern in einem Satz: kein neues Trust-System, sondern die vorhandene dritte Trust-Schicht von „statische Tabelle" auf „Tabelle UND Agreement-Credential UND optionaler lokaler PDP" erweitern — mit ungesetzter PDP-URL als exaktem Status quo.

---

## Quellen

- Gaia-X Trust Framework 22.10 (historisch) — https://docs.gaia-x.eu/policy-rules-committee/trust-framework/22.10/participant/
- Gaia-X Compliance Document 25.10 (Loire, aktuell) — https://docs.gaia-x.eu/policy-rules-committee/compliance-document/25.10/criteria_participant/ · Prozess: https://gaia-x.gitlab.io/policy-rules-committee/compliance-document/Process/
- Gaia-X Architecture Document 24.04, Digital Clearing House — https://docs.gaia-x.eu/technical-committee/architecture-document/24.04/digital_clearing_house/
- "How to become a Gaia-X conformant Service" — https://gaia-x.eu/wp-content/uploads/2024/10/How-to-become-a-Gaia-X-conformant-Service.pdf
- GXDCH Live-APIs — https://registry.gaia-x.eu/v1/docs/ · https://compliance.gaia-x.eu/v1/docs/
- XFSC Trust Services API Extension 1 (TSA) — https://eclipse.dev/xfsc/tsae1/tsae1/ · Referenzimplementierung (GitLab, archiviert): https://gitlab.eclipse.org/eclipse/xfsc/tsa/policy · Nachfolger: https://github.com/eclipse-xfsc/custom-policy-agent
- XFSC TRAIN — https://eclipse.dev/xfsc/train/train/ · Repos: https://github.com/eclipse-xfsc (train-*)
- XFSC Notarization API Extension — https://eclipse.dev/xfsc/notare/notare/
- Eclipse XFSC Projektstatus — https://projects.eclipse.org/projects/technology.xfsc · FACIS/IPCEI-CIS: https://github.com/eclipse-xfsc/facis
- Decentralized Claims Protocol (DCP) v1.0.1 — https://eclipse-dataspace-dcp.github.io/decentralized-claims-protocol/ · trust.model.md / base.protocol.md / verifiable.presentation.protocol.md: https://github.com/eclipse-dataspace-dcp/decentralized-claims-protocol
- Eclipse Dataspace Components — https://github.com/eclipse-edc/Connector · https://github.com/eclipse-edc/IdentityHub · https://github.com/eclipse-edc/MinimumViableDataspace
- Tractus-X EDC Policies — https://github.com/eclipse-tractusx/tractusx-edc/blob/main/docs/usage/management-api-walkthrough/02_policies.md · Catena-X CX-0006 — https://catenax-ev.github.io/docs/next/standards/CX-0006-RegistrationAndInitialOnboarding
- W3C Verifiable Credentials Data Model 2.0 (Recommendation, 15.05.2025), § 5.5 Terms of Use — https://www.w3.org/TR/vc-data-model-2.0/#terms-of-use
- Verordnung (EU) 2024/1183 (eIDAS 2.0), Volltext via EU Publications Office — https://eur-lex.europa.eu/eli/reg/2024/1183/oj (Volltext-Extraktion: http://publications.europa.eu/resource/celex/32024R1183)
- Verordnung (EU) 910/2014 (eIDAS), Art. 25/26 — https://eur-lex.europa.eu/eli/reg/2014/910/oj
