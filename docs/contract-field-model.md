# Contract field model

Status: Accepted (2026-07-24).

## Decision

Canonical templates and contracts separate field declarations from business
data:

- `dcs:contractFields` is a flat list of `dcs:ContractField` nodes. Every field
  has `@id`, `dcs:label`, `dcs:datatype`, and boolean `dcs:required`; it may
  additionally carry `dcs:shape` and, on a filled contract, `dcs:value`.
- `dcs:contractData` contains typed domain objects such as
  `dcs:PaymentClause`. Their business properties bind to fields using bare
  `{"@id":"…"}` references.
- `dcs:documentStructure` is the human-readable structure. Layout children use
  `{"@list":[{"@id":"…"}]}` and clause content uses only bare field references.
- ODRL operands reference the same field identifiers.

Example:

```json
{
  "@type": "dcs:Contract",
  "dcs:contractFields": [
    {
      "@id": "#payment-amount",
      "@type": "dcs:ContractField",
      "dcs:label": "Payment amount",
      "dcs:datatype": "xsd:decimal",
      "dcs:required": true,
      "dcs:value": 15000
    }
  ],
  "dcs:contractData": [
    {
      "@id": "#payment",
      "@type": "dcs:PaymentClause",
      "dcs:amount": {"@id": "#payment-amount"}
    }
  ]
}
```

The document is self-contained: authoring, validation, rendering, and policy
evaluation resolve a field by `@id` without consulting a template snapshot.
