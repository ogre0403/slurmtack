## ADDED Requirements

### Requirement: Selected execution detail refreshes while polling

The dashboard SHALL refresh the currently selected execution detail view on the same polling cadence used for execution history while the detail drawer remains open. When the selected execution advances, fails, or completes between polls, the dashboard MUST refresh both the execution summary and the step timeline without requiring the operator to reselect that execution.

#### Scenario: Selected allocation wait updates to failed state

- **WHEN** the operator has selected an execution currently shown as `awaiting_source_allocation`
- **AND** a later poll finds that execution transitioned to `failed_non_destructive`
- **THEN** the dashboard updates the detail panel to show the failed current state and overall status
- **AND** it renders the failed `wait_for_source_allocation` step and its recorded failure summary without requiring the operator to reopen the panel
