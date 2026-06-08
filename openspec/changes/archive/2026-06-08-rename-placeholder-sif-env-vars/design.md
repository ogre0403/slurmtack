## Context

The current SIF configuration contract was introduced when the runtime image was still framed as a placeholder-specific artifact. Today the daemon still reads `PLACEHOLDER_SIF_PATH` and `PLACEHOLDER_SIF_FILE` in `internal/config/config.go`, request validation still tells operators to configure `PLACEHOLDER_SIF_FILE` in `internal/service/switch.go`, the Slurm client resolves runtime paths from the same config, and README/build output still document the old names.

This change is cross-cutting because the environment variable names are part of the daemon deployment contract, spec text, operator documentation, startup validation, and request-time error handling. The underlying semantics are already correct: the path is a home-relative directory and the file is a default basename. The problem is naming, not runtime behavior.

## Goals / Non-Goals

**Goals:**
- Rename the daemon environment variables to `SLURM_SIF_PATH` and `SLURM_SIF_FILE` everywhere they are part of the supported contract.
- Preserve current behavior for home-relative path validation, default filename fallback, and per-request `placeholder_sif_file` override.
- Update all operator-facing docs and error messages so deployments can migrate cleanly.

**Non-Goals:**
- Renaming API request fields such as `placeholder_sif_file`.
- Renaming persisted execution fields, database columns, or internal placeholder-agent concepts that are not part of the env contract.
- Changing how the runtime SIF path is resolved or adding new fallback rules beyond the env rename.

## Decisions

### 1. Rename the external env contract without widening scope

**Choice:** The daemon will stop using `PLACEHOLDER_SIF_PATH` and `PLACEHOLDER_SIF_FILE` as supported configuration names and instead load `SLURM_SIF_PATH` and `SLURM_SIF_FILE`.

**Why:** The request is specifically about env renaming, and these variables are operator-facing configuration. Renaming only the external contract keeps the change small and predictable while still removing the outdated placeholder terminology from deployment surfaces.

**Alternatives considered:**
- Keep old names as permanent aliases: rejected because it preserves an ambiguous public contract and forces docs/specs to explain two names for one setting.
- Rename every internal identifier from `PlaceholderSIF*` to `SlurmSIF*`: rejected because it expands the blast radius into API, persistence, and internal domain terms that are not required to satisfy the config rename.

### 2. Keep path semantics and filename validation unchanged

**Choice:** `SLURM_SIF_PATH` continues to mean a workload-home-relative directory, and `SLURM_SIF_FILE` continues to mean the default filename used when the request omits `placeholder_sif_file`.

**Why:** Operators are asking for a clearer name, not a new runtime model. Preserving semantics avoids unnecessary migration complexity and keeps existing request and execution behavior stable.

**Alternatives considered:**
- Reinterpret `SLURM_SIF_PATH` as an absolute path or full file path: rejected because it would undo the current per-user home-directory design and require broader spec changes.

### 3. Align all operator-visible failure text with the new names

**Choice:** Startup validation errors, request-time missing-default guidance, README examples, and build/deployment messaging will all reference `SLURM_SIF_PATH` and `SLURM_SIF_FILE`.

**Why:** Leaving old names in diagnostics would create a migration trap where the implementation accepts new names but tells operators to fix old ones.

**Alternatives considered:**
- Update only env loading and leave existing messages intact: rejected because it creates inconsistent operational guidance and makes troubleshooting harder.

## Risks / Trade-offs

- [Existing deployments still set the old env names] → Mitigation: mark the rename as breaking in the proposal/specs and update README plus env examples with direct old-to-new migration guidance.
- [Partial rename leaves stale operator guidance] → Mitigation: include docs, build script output, validation errors, and spec deltas in the same change scope.
- [Future contributors assume API field names also changed] → Mitigation: design and tasks explicitly state that `placeholder_sif_file` remains unchanged and only the daemon env contract is renamed.

## Migration Plan

1. Update daemon config loading and validation to read `SLURM_SIF_PATH` / `SLURM_SIF_FILE`.
2. Update request validation and Slurm submission paths to use the renamed config fields while preserving the existing runtime behavior.
3. Update README, env examples, build output, and specs so deployment guidance references only the new names.
4. Roll out by renaming the two env vars in deployment manifests before restarting the daemon.

Rollback strategy: restore the previous binary/spec state and rename deployment env vars back to `PLACEHOLDER_SIF_PATH` / `PLACEHOLDER_SIF_FILE`.

## Open Questions

- None. The requested scope is clear and the current behavior is already defined by existing specs.
