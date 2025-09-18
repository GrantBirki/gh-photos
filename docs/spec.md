# Feature Specification: GitHub CLI Extension for iPhone Photo Backup to Google Drive

**Input**: User description: "A cross-platform Go CLI (distributed as a gh CLI extension) that ingests an unencrypted iPhone backup folder, parses the Photos database to identify asset types (photos, videos, screenshots, burst, Live Photos), excludes assets flagged Hidden or Recently Deleted by default (but optionally includes them via flags), organizes files into a `photos/<YYYY>/<MM>/<DD>/<category>/` hierarchy (Categories: `photos`, `videos`, `screenshots`, `burst`, `live_photos`), optionally deduplicates by checksum, and then uploads/syncs them to an rClone remote (Google Drive, S3, etc.). The tool must be cloud-agnostic on output (rClone remote only) and iPhone-specific on input. It must support dry-run, skip-existing, verify, verbose/logging, manifest saving, parallel upload tuning, date filters, and type filters. Users install via GH CLI extension; prefer Cobra for CLI flag handling. The default behaviour: do not upload Hidden or Recently Deleted assets."

## Execution Flow (main)

```text
1. Parse user description from Input
   → If empty: ERROR "No feature description provided"
2. Extract key concepts from description
   → Identify: actors, actions, data, constraints
3. For each unclear aspect:
   → Mark with [NEEDS CLARIFICATION: specific question]
4. Fill User Scenarios & Testing section
   → If no clear user flow: ERROR "Cannot determine user scenarios"
5. Generate Functional Requirements
   → Each requirement must be testable
   → Mark ambiguous requirements
6. Identify Key Entities (if data involved)
7. Run Review Checklist
   → If any [NEEDS CLARIFICATION]: WARN "Spec has uncertainties"
   → If implementation details found: ERROR "Remove tech details"
8. Return: SUCCESS (spec ready for planning)
```

## User Scenarios & Testing (mandatory)

### Primary User Story

As a technically proficient user with an unencrypted iPhone backup and no iCloud subscription, I want a single cross-platform CLI that:

reads the iPhone Photos.sqlite metadata and the DCIM files,

classifies assets into Photos, Videos, Screenshots, Burst, and LivePhotos,

excludes Hidden and Recently Deleted assets by default (but lets me opt in),

organizes the final export into `photos/<YYYY>/<MM>/<DD>/<category>/`,

optionally deduplicates files by checksum,

syncs/upload to an rClone remote (e.g., Google Drive) so I can later import into Google Photos,
so that I can automate backups reliably, avoid uploading private items, and keep the cloud layout tidy and importable.

### Acceptance Scenarios

Given an unencrypted iPhone backup folder with Photos.sqlite and DCIM files, When the user runs gh iphone-backup sync --backup /path/to/backup --remote gdrive:Photos (no extra flags), Then the tool:

parses Photos.sqlite,

excludes any assets flagged Hidden or Recently Deleted,

categorizes remaining assets,

maps each asset to a path like `photos/2025/09/17/photos/IMG_0001.HEIC`,

generates an rClone upload plan,

and executes the upload.

Given the same backup and the user runs with --dry-run, When the tool runs, Then it prints the full planned mapping and upload actions but does not call rClone.

Given large asset sets with duplicates, When the user runs with --skip-existing and the remote already contains matching checksums, Then the tool skips those uploads.

Given the user wants Hidden files uploaded, When they pass --include-hidden, Then assets flagged Hidden are treated like normal assets and included in the plan.

Given a date filter --start-date=2024-01-01 --end-date=2024-12-31, When the tool runs, Then only assets whose creation timestamp is within that inclusive range are considered/uploaded.

### Edge Cases

What happens if Photos.sqlite references an asset whose raw file is missing from DCIM? → tool must log the missing asset and continue; mark for user review.

How are Live Photos represented when the still and the companion motion file exist separately? → tool must detect and preserve pairing or clearly document how it places the files. [NEEDS CLARIFICATION: expected user preference for storing Live Photo pairs — keep together in `live_photos/` folder, or keep HEIC in `photos/` and MOV in `videos/`?]

Multiple assets with same filename but different contents (collisions) in same target folder → tool must avoid overwrite by filename collision policy (e.g., content-based renaming or suffix). [NEEDS CLARIFICATION: preferred collision policy? e.g., append hash, incrementing suffix, or fail?]

Files with no extension or non-standard extensions → tool should infer type by metadata and normalize target filenames. [NEEDS CLARIFICATION: normalization rules for filenames across providers?]

Very large folders on certain remotes may cause slow listing or UI issues (e.g., thousands of files in one directory). Tool should default to date-based nested folders to mitigate this.

Timezone/EXIF inconsistencies: asset creation vs device timezone may conflict; the tool must document how it determines date (EXIF creation time vs DB timestamp). [NEEDS CLARIFICATION: canonical source of date for folder placement — Photos.sqlite ZCREATIONDATE or file EXIF?]

## Requirements (mandatory)

### Functional Requirements

- FR-001: The system MUST accept a path to an unencrypted iPhone backup folder and parse the Photos database to enumerate assets.
- FR-002: The system MUST classify every asset into one of these categories: Photos, Videos, Screenshots, Burst, LivePhotos.
- FR-003: The system MUST default to excluding assets flagged Hidden and Recently Deleted in the Photos database.
- FR-004: The system MUST expose flags to override default exclusion:

    --include-hidden to include Hidden assets.

    --include-recently-deleted to include Recently Deleted assets.

- FR-005: The system MUST map assets to the folder structure `ROOT/<YYYY>/<MM>/<DD>/<category>/filename.ext` where ROOT is user-configurable (via --remote prefix mapping).

- FR-006: The system MUST support a --remote argument that accepts an rClone remote target and use rClone to perform the sync/upload.
- FR-007: The system MUST provide a --dry-run mode that prints the planned operations without performing uploads.
- FR-008: The system MUST provide --skip-existing to skip upload of files already present on the remote (using content checksum or rClone's existing behavior). [NEEDS CLARIFICATION: exact check method — rely on rClone remote metadata, or precompute local hashes and compare?]
- FR-009: The system MUST provide --verify to optionally validate post-upload integrity (checksum comparison).
- FR-010: The system MUST provide --save-manifest to write a JSON manifest enumerating: source path, target path, classification, timestamp, checksum.
- FR-011: The system MUST provide logging options: --verbose, --log-file, and --log-level (debug, info, warn, error).
- FR-012: The system MUST accept --parallel to tune concurrency for uploads.
- FR-013: The system MUST accept --start-date and --end-date filters (YYYY-MM-DD) to constrain assets by creation date.
- FR-014: The system MUST accept --type as a comma-separated, lowercase list (photos, videos) to include only requested asset classes.
- FR-015: The system MUST omit sidecar files (e.g., .aae) and symlinks from uploads by default.
- FR-016: The system MUST handle missing files gracefully (log and continue) and present a summary at the end (uploaded, skipped, failed, missing).
- FR-017: The system MUST be cross-platform (macOS, Linux, Windows) in CLI behavior and path handling from the user perspective.
- FR-018: The system MUST produce deterministic folder mapping (same input backup → same target layout) so subsequent runs are idempotent when --skip-existing is used.

Unclear / ambiguous requirements flagged:

- FR-019: Deduplication method MUST be defined. [NEEDS CLARIFICATION: user prefers checksum-based deduplication (SHA256) — confirm algorithm and whether perceptual dedupe is desired.]
- FR-020: Live Photo pairing and folder assignment behavior. [NEEDS CLARIFICATION: should Live Photo HEIC and MOV be co-located in `live_photos/`, or split into `photos/` and `videos/`?]
- FR-021: Filename collision policy on target. [NEEDS CLARIFICATION: append hash / increment / fail?]

Non-Functional Requirements

- NFR-001: The system MUST not upload Hidden or Recently Deleted assets unless explicitly requested by flags.
- NFR-002: The system SHOULD preserve original file modification timestamp and EXIF metadata where possible.
- NFR-003: The system SHOULD avoid creating extremely large single directories (use date-based nesting by default).
- NFR-004: The system SHOULD run efficiently on commodity developer machines; expensive operations (e.g., hashing all files) should be optional flags.
- NFR-005: The tool SHOULD document any provider-specific limits (e.g., filename restrictions) and sanitize filenames when necessary. [NEEDS CLARIFICATION: which sanitization rules to apply per remote?]

Key Entities (include if feature involves data)

Backup: Represents the unencrypted iPhone backup folder. Key attributes: path, Photos.sqlite path, DCIM root.

Asset: A single photo/video record from Photos.sqlite. Key attributes: source file path, media type, creation timestamp, flags (Hidden, RecentlyDeleted, LivePhoto, BurstID, Screenshot), checksum (optional).

Category: One of Photos, Videos, Screenshots, Burst, LivePhotos.

ManifestEntry: An entry in the exported JSON manifest: { sourcePath, targetPath, category, timestamp, checksum, status }.

UploadPlan: The ordered list of manifest entries with decisions (skip, upload, verify).

Config: CLI options and defaults (includeHidden, includeDeleted, parallel, remote, rootPrefix, etc.)

## Review & Acceptance Checklist

GATE: Automated checks run during main() execution

## Content Quality

- No low-level code examples (kept at feature level)
- Focused on user value and business needs
- Written for stakeholders and developers
- All mandatory sections completed

### Requirement Completeness

- No [NEEDS CLARIFICATION] markers remain
- Requirements are testable and unambiguous
- Success criteria are measurable
- Scope is clearly bounded
- Dependencies and assumptions identified

Current status: There are open [NEEDS CLARIFICATION] items flagged (dedupe algorithm, live photo pairing, filename collision policy, checksum vs remote compare, sanitization rules per remote). These must be resolved before marking the checklist as fully complete.

## Execution Status

- [ ] Updated by main() during processing
- [ ] User description parsed
- [ ] Key concepts extracted
- [ ] Ambiguities marked
- [ ] User scenarios defined
- [ ] Requirements generated
- [ ] Entities identified
- [ ] Review checklist passed

Design Constraints / Assumptions (requested by stakeholder)

These are explicit constraints the requester asked to keep, recorded here as assumptions to guide planning. These are not implementation instructions but boundary choices.

Input is iPhone-only: the tool must parse Photos.sqlite from unencrypted iPhone backups; Android is out of scope.

Output is cloud-agnostic: the tool will use an rClone remote argument; the tool itself will not call provider APIs directly.

The stakeholder prefers a Go implementation distributed as a GitHub CLI extension and suggested using Cobra for flag parsing. (These are implementation preferences — include as assumptions to be honored by engineering.)

Default privacy settings: Hidden = excluded; Recently Deleted = excluded. Flags allow opt-in.

Primary target UX: fully automated CLI (scriptable), with --dry-run and manifests for auditing.

Open Questions / [NEEDS CLARIFICATION]

Dedupe algorithm — Do we use SHA256 for exact-content dedupe only, or support later perceptual hashing (pHash/dHash) for near-duplicates? (Spec currently expects exact checksum dedupe; confirm.)

Live Photo pairing policy — How should Live Photo still+motion pairs be represented in the target layout? (Keep both in `live_photos`/ adjacent, or put stills into `photos/` and motion into `videos/`?)

Filename collision policy — On name collision in the same target folder, should the tool append a content hash suffix, an incrementing numeric suffix, or skip/upload and fail?

Date canonicalization — Use Photos.sqlite ZCREATIONDATE, or fallback to EXIF DateTimeOriginal / file timestamp if missing? Which takes precedence?

Remote-specific filename sanitization — Should the tool apply a universal sanitization policy (safe ASCII, remove :, ?, etc.), or adapt rules per rClone backend?

Manifest retention & size — How long to retain local manifests, and where to store them? (local disk default).

Behavior on partial uploads/failures — Retry strategy and rollback semantics (if any). Define expected behavior for intermittent network failures.

Performance policy — Default --parallel=4 suggested; confirm acceptable defaults and whether to limit concurrently spawned rClone processes vs single rClone with internal parallelism.

Google Photos import — The user currently plans manual import from Drive → Google Photos. Should the tool provide helper instructions or produce an export layout optimized for Google Photos import? (E.g., flatten vs date folders.)

Security / encryption — Should the tool optionally support client-side encryption (e.g., via Cryptomator) before upload? [NEEDS CLARIFICATION]

Next steps (recommended)

Resolve the Open Questions above with the stakeholder (answer the [NEEDS CLARIFICATION] items).

Convert finalized spec into acceptance tests / user stories (small tasks), e.g., one task per FR.

Create a minimal MVP plan:

MVP scope: parse Photos.sqlite, classify assets, generate manifest, support --dry-run, map to `YYYY/MM/DD/<category>`, produce rClone upload commands (dry-run mode only).

After MVP: add actual rClone invocation, --skip-existing, --verify, --parallel, and robust error handling.

Prepare developer onboarding note that confirms: the stakeholder requested Go + GH CLI extension + Cobra — list of repo structure tips and where to place the manifest and log output.
