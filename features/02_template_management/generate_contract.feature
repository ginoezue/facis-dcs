@UC-02-03
Feature: Generate Contract from Template
  Contract Creators generate contract instances
  by populating approved templates with data.
  
  # contract/create only accepts templates in state REGISTERED or PUBLISHED
  # (backend ReadContractTemplateDataByID) — APPROVED alone is not enough.
  Scenario: Generate contract from approved template
    Given I am authenticated with roles: "Contract Creator"
    And template "Standard NDA" is in "Registered" status
    When I generate a contract from template "Standard NDA"
    Then a contract is created

  @REQ-remove-document-number-AC2 @DCS-FR-CWE-13 @DCS-FR-CSA-09
  Scenario: Generated contracts contain no legacy document number
    Given a registered template with a legacy document number exists
    When I generate a contract from that legacy template
    Then the contract keeps its template reference without a document number

  Scenario: Unauthorized role cannot generate contract
    Given I am authenticated with roles: "Template Approver"
    And template "Standard NDA" is in "Registered" status
    When I generate a contract from template "Standard NDA"
    Then the request is denied with an authorization error
