## MODIFIED Requirements

### Requirement: Browse execution history and inspect step timelines

The dashboard SHALL provide an execution-focused right-side panel with a paginated execution list and a drilldown panel for an individual execution. The dashboard MUST offer node, status, direction, and requested-date-range filters for that execution list. On initial load, the requested-date-range filter MUST default to the operator's local calendar dates from seven days before today through today. The execution list MUST include recent active executions and prior executions returned by the API that match the selected filters, show 10 executions per page by default, and display each execution's direction, node binding when known, `current_state`, `overall_status`, and request time. Selecting an execution MUST show the execution summary and current state in the detail panel, and the drilldown panel MUST render the ordered step timeline returned for the selected execution with fine-grained metadata for each step, including step name, sequence, status, started time, ended time when present, host when present, retry count, exit code or failure classification when present, step `error_summary` when present, and any recorded output or snapshot paths as secondary detail. Changing any execution-list filter, including the requested date range, MUST refresh the execution list and reset pagination to the first page.

#### Scenario: Execution history defaults to the recent seven-day window

- **WHEN** the operator opens the dashboard execution panel
- **THEN** the dashboard initializes the requested-date-range filter to the local date seven days before today through today
- **AND** it requests only executions inside that default window

#### Scenario: Inspect execution details from the execution list

- **WHEN** the operator selects an execution from the right-side execution list
- **THEN** the dashboard shows the execution summary, its current state, and its ordered step records in the detail panel
- **AND** each rendered step shows at least its name, status, and timing metadata

#### Scenario: Running step remains visible while execution is in progress

- **WHEN** the selected execution includes a step whose `status` is `running`
- **AND** that step does not yet have `ended_at`
- **THEN** the dashboard renders that step as the current in-progress step instead of hiding it
- **AND** the rest of the persisted timeline remains visible around it

#### Scenario: Failed step surfaces operator-relevant failure detail

- **WHEN** the selected execution timeline includes a step with `status=failed`
- **THEN** the dashboard shows that step's `error_summary` when present
- **AND** it also shows `error_class` or `exit_code` when present
- **AND** it also shows any recorded stdout, stderr, or snapshot path metadata when present

#### Scenario: Blocked precheck shows readable refusal reason

- **WHEN** the selected execution timeline includes a failed precheck step classified as `precheck_blocked`
- **THEN** the execution detail view shows the operator-visible refusal reason from that step without requiring the operator to inspect external logs

#### Scenario: Browse the next page of executions

- **WHEN** the execution list contains more than 10 matching executions inside the selected filters
- **AND** the operator navigates to the next page
- **THEN** the dashboard shows the next 10 executions from the list query
- **AND** it keeps the execution rows ordered consistently with the API response

#### Scenario: Browse the previous page of executions

- **WHEN** the operator has navigated beyond the first execution page
- **AND** the operator navigates back to the previous page
- **THEN** the dashboard restores the immediately preceding set of 10 execution rows

#### Scenario: Active execution remains actionable in the list

- **WHEN** the execution list includes an execution whose `overall_status` is `active`
- **THEN** the row shows the execution's current state
- **AND** the row includes a cancel control without requiring the operator to open the detail panel first

#### Scenario: Filter execution list by node, outcome, direction, or requested date range

- **WHEN** the operator applies a filter such as `node=gpu-01`, `status=failed`, or a narrower requested date range
- **THEN** the dashboard refreshes the execution list using those filters
- **AND** it resets the paginated view to the first page of matching executions
