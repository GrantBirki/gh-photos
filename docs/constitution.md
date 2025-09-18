# Core Principles

## I. Library-First, CLI-Friendly

Every feature MUST be implemented as a small, well-documented, independently testable Go package (library) that exposes its functionality through a CLI-facing command. Libraries are the primary unit of design; the CLI is a thin, well-documented adapter that composes libraries into user-facing flows.

- Libraries must have a single responsibility (parsing, categorizing, hashing, rClone orchestration, manifest generation, logging).
- Libraries must expose a programmatic API suitable for unit tests and for future reuse (e.g., a GUI or API server).
- CLI commands (Cobra) are glue: parse flags → call library APIs → format output.

## II. Predictable Text I/O & Human+Machine Readable Surface

All CLI behavior must be predictable and scriptable.

- Primary input/output protocol: CLI args + stdin → stdout (human readable and JSON) and errors → stderr.
- Important outputs (manifests, plans, summaries) MUST be available in JSON for automation and auditing.
- `--dry-run` must produce a machine-readable plan (JSON) and a human summary.

## III. Test-First (Non-Negotiable)

TDD is required for all production libraries and critical CLI flows.

- For each new feature or bugfix, write failing unit tests and integration tests before implementing behavior.
- Unit tests for parsing Photos.sqlite, classification logic, date normalization, filename sanitization, dedupe logic, and rClone interaction wrapper.
- Integration tests for end-to-end flows should run in CI using small sample backups (sanitized fixtures), exercising `--dry-run`, manifest generation, and error handling.

## IV. Integration & Contract Testing

Focus integration tests on contracts between modules and external tooling.

- Contracts to test: Photos.sqlite schema parsing → Asset entity, Manifest schema, rClone CLI wrapper contract (commands produced), and filesystem interactions.
- Add lightweight contract tests for cross-platform path handling (Windows/macOS/Linux), and for rClone command shapes (not provider internals).
- Any change to a library contract must include automated contract tests and migration notes.

## V. Observability, Simplicity & Backward Compatibility

Operate with clear logs, small surface area, and careful change management.

- Structured logging required (JSON or leveled text) with `--log-level` support (`debug`, `info`, `warn`, `error`).
- Keep defaults safe and simple: Hidden = excluded, Recently Deleted = excluded, date-nested folders by default, `--dry-run` off by default.
- Use semantic versioning (MAJOR.MINOR.PATCH). Breaking changes require a migration plan and a documented upgrade path.
- Keep the codebase YAGNI-lean — add complex features (perceptual dedupe, encryption) behind opt-in flags and separate modules.

---

## Security, Privacy & Compliance Requirements

- **Default privacy posture:** Do not expose or upload Hidden or Recently Deleted assets unless explicit flags are provided (`--include-hidden`, `--include-recently-deleted`).
- **No provider credentials stored by default:** rClone handles provider auth. The tool must not persist provider credentials; if any credentials must be stored (temporary), store only per OS best practices and document it explicitly.
- **Sidecars and sensitive files:** By default omit sidecar files (e.g., `.aae`) and symlinks. Provide an opt-in flag only if needed.
- **Client-side encryption optional:** The tool may document and optionally integrate with client-side encryption tools (e.g., Cryptomator) but must not enable encryption by default. Any encryption integration must be opt-in and have explicit key management documentation.
- **Logs & manifests:** Manifests may contain personal data (paths, timestamps). The default retention is local only; persist manifests only when `--save-manifest` is provided or when `--log-file` is used. Document retention practices and allow manual deletion.
- **Least privilege & local operations:** All parsing and transformations happen locally against the unencrypted backup; the tool should not call external services except rClone (which the user configures).

---

## Performance & Operational Constraints

- Default concurrency is `--parallel=4`. Expensive operations (global hashing of the entire library) must be explicit flags (`--checksum`, `--prehash`).
- The tool must favor incremental operations: maintain a manifest/cache so subsequent runs with `--skip-existing` can be efficient.
- Avoid creating huge single folders by default: use `YYYY/MM/DD/<Category>` layout.
- Provide robust retry behavior for transient failures (retry with exponential backoff for rClone failures) and graceful resumability.
- File IO and hashing must be stream-friendly (do not load entire large files in memory).

---

## Development Workflow & Quality Gates

- **Branching:** Feature development occurs on feature branches named with the pattern `###-short-description` where `###` is the issue/PR number.
- **PR requirements:**
  - Each PR must include unit tests demonstrating the change.
  - All CI checks must pass, including unit tests, linters, and basic integration tests (`--dry-run` flows against sample fixtures).
  - Any PR touching library contracts must include migration notes and update contract tests.
- **Code review:** At least one reviewer must sign off; reviewers must confirm:
  - No secrets or provider credentials are committed.
  - Tests exist for new behavior and edge cases.
  - Logging and error messages are informative and respect privacy.
- **Release:** Semantic version bump, changelog entry, and a short migration note if applicable.
- **CI:** Runs on Linux/macOS/Windows runners for critical integration tests (path handling, rClone command generation). rClone binary is assumed installed in CI or its behavior mocked for tests.

---

## Project Constraints & Technology Assumptions

- Implementation language: **Go**.  
- CLI framework: **Cobra** (used for `gh` extension integration).  
- Output layer: **rClone** (tool calls rClone CLI for cloud-agnostic uploads). The tool must generate rClone commands and call rClone as a subprocess; direct rClone imports or vendorization are disallowed unless a stable, supported library API exists and is approved by maintainers.
- Input: **Unencrypted iPhone backups only** (Photos.sqlite + DCIM). Android/other phones out of scope.
- Cross-platform support: macOS, Linux, Windows. Use `filepath` utilities and test fixtures for each platform.
- The tool will not call cloud provider APIs directly; the `--remote` flag accepts rClone remote syntax (e.g., `gdrive:Photos`, `s3:bucket/path`).

---

## Governance

- This Constitution is the source of truth for project practices. Any amendment requires:
  - A documented rationale in the repo (PR) and at least two maintainer approvals.
  - A migration plan for any changes that affect public CLI behavior or stored manifest formats.
- All PRs must include a checklist referencing this Constitution (privacy, tests, logging, cross-platform).
- Complexity must be justified: new dependencies require security review and approval.
- Sensitive design decisions (encryption integration, perceptual dedupe, auto-import into Google Photos) must be discussed in a dedicated RFC issue and documented before implementation.

**Version**: 1.0.0 | **Ratified**: 2025-09-17 | **Last Amended**: 2025-09-17
