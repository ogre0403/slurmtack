## 1. Logging Foundation

- [x] 1.1 Add a base `log/slog` logger in `cmd/main.go` and thread logger dependencies into the switch runtime path constructors used by `service`, `engine`, `orchestrator`, and `mq`.
- [x] 1.2 Add a shared execution-trace helper that derives execution-scoped loggers and standardizes core fields and event names.
- [x] 1.3 Update affected constructor call sites and tests so injected loggers remain optional or have safe defaults where needed.

## 2. Workflow Instrumentation

- [x] 2.1 Instrument `service.SwitchService.RequestSwitch`, `engine.Runner.Transition`, and `engine.Runner.FailExecution` to log request acceptance, transition attempts and outcomes, and failure classification.
- [x] 2.2 Instrument `engine.RunStep` and existing handler-based flows so step start and end logs use the same execution-scoped trace vocabulary.
- [x] 2.3 Instrument orchestrator action selection and action execution paths, including placeholder submission, lease acquisition, precheck, quiesce, reconfigure, reboot, attach, verify, completion, and terminal failure handling.
- [x] 2.4 Instrument asynchronous wait paths in `mq.Consumer` and `orchestrator.PollSSHReachable` so allocation waits, drained waits, and reboot reachability polling emit entered, progress, and satisfied or timeout logs.

## 3. Validation

- [x] 3.1 Add focused tests that capture logger output and verify required trace events and fields for success and failure cases in the engine and orchestrator.
- [x] 3.2 Extend representative integration coverage so switch workflows assert trace logs for action selection, asynchronous waits, and terminal completion or failure.
- [x] 3.3 Update relevant operational or developer documentation to describe the new trace fields and where to inspect them during switch debugging.