## 1. Execution Panel State

- [x] 1.1 Replace the current history "load more" state with execution pagination state that defaults to 10 items per page and resets to page 1 when filters change.
- [x] 1.2 Update execution list fetching to keep current state, status, and request metadata visible in each row while preserving selection of the active execution detail view.
- [x] 1.3 Refresh the current execution page and selected execution detail after cancellation so accepted state transitions are visible without a full page reload.

## 2. Dashboard UI

- [x] 2.1 Rework the right-side panel markup and styling to present an execution-focused list with page navigation controls instead of the current load-more history pattern.
- [x] 2.2 Add inline cancel controls for active execution rows and keep node-card cancellation behavior aligned with the same API flow.
- [x] 2.3 Ensure clicking an execution row opens details that prominently show the current state before the step timeline.

## 3. Verification

- [x] 3.1 Update dashboard HTML/JS tests to cover the execution-centric panel elements, pagination controls, and execution-detail state rendering.
- [x] 3.2 Add or adjust dashboard interaction coverage for active execution cancellation from the right-side panel.
- [x] 3.3 Run the relevant dashboard/UI test suite and confirm the new execution panel behavior matches the spec.
