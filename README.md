# gh-photos 📸

A gh cli extension for uploading iPhone photos from a backup dump to a remote target using rclone and logical defaults.

[![test](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml)
[![build](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml)
[![lint](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml)
[![release](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml)
![slsa-level3](docs/assets/slsa-level3.svg)

## About ⭐

This project is a [`gh cli`](https://github.com/cli/cli) extension that extracts photos and videos from unencrypted iPhone backup directories and uploads them to cloud storage using [rclone](https://rclone.org/). The tool intelligently parses the `Photos.sqlite` database to classify assets by type and organizes them into a clean folder structure on your chosen cloud provider.

**Key Features:**

- 📱 Extracts photos from unencrypted iPhone backups
- 🗂️ Organizes uploads by date and asset type (`photos/YYYY/MM/DD/<category>/`)
- 🔐 Privacy-safe defaults (excludes Hidden/Recently Deleted albums)
- ☁️ Supports all **rclone** remotes (Google Drive, S3, OneDrive, etc.)
- ⚡ Parallel uploads for faster performance
- 🔍 Asset classification (photos, videos, screenshots, burst, Live Photos)
- 📋 Manifest generation for operation auditing
- 🧪 Dry-run mode for safe testing

## Installation 💻

Install this gh cli extension by running the following command:

```bash
gh extension install grantbirki/gh-photos
```

### Upgrading 📦

You can upgrade this extension by running the following command:

```bash
gh ext upgrade photos
```

## Usage 🚀

The `gh-photos` extension provides three main commands for working with iPhone backup photos:

### Basic Commands

```bash
# Sync photos from iPhone backup to Google Drive
gh photos sync /path/to/backup gdrive:photos

# Dry run to preview what would be uploaded
gh photos sync /path/to/backup gdrive:photos --dry-run

# Validate an iPhone backup directory
gh photos validate /path/to/backup

# List assets found in backup
gh photos list /path/to/backup

# Upload to nested folder structure on the remote - This creates: Google Drive/Backups/iPhone/2024/YYYY/MM/DD/
gh photos sync /path/to/backup gdrive:Backups/iPhone/2024

# This creates: Google Drive/Photos/family-photos/YYYY/MM/DD/
gh photos sync /backup gdrive:Photos --root "family-photos"
```

### Advanced Usage Examples

```bash
# Sync with custom settings
gh photos sync /backup gdrive:photos \
  --include-hidden \
  --parallel 8 \
  --checksum \
  --save-manifest manifest.json

# Filter by date range and asset types
gh photos sync /backup s3:mybucket/photos \
  --start-date 2023-01-01 \
  --end-date 2023-12-31 \
  --types photos,videos

# Upload to custom root directory
gh photos sync /backup gdrive:photos \
  --root "family-photos" \
  --skip-existing \
  --verify

# List assets with filtering
gh photos list /backup \
  --include-hidden \
  --types screenshots,burst \
  --format json
```

### Command Line Options

#### Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--no-color` | Disable colored output | `false` |
| `--verbose` | Enable verbose logging | `false` |
| `--log-level` | Set log level (debug, info, warn, error) | `info` |
| `--help` | Show help for command | - |
| `--version` | Show version information | - |

#### Sync Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--include-hidden` | Include assets flagged as hidden | `false` |
| `--include-recently-deleted` | Include assets flagged as recently deleted | `false` |
| `--dry-run` | Preview operations without uploading | `false` |
| `--skip-existing` | Skip files that already exist on remote | `false` |
| `--verify` | Verify uploaded files match source | `false` |
| `--checksum` | Compute SHA256 checksums for assets | `false` |
| `--parallel` | Number of parallel uploads | `4` |
| `--root` | Root directory prefix for uploads | `photos` |
| `--save-manifest` | Path to save operation manifest (JSON) | - |
| `--types` | Asset types to include (photos,videos,screenshots,burst,live_photos) | all |
| `--start-date` | Start date filter (YYYY-MM-DD) | - |
| `--end-date` | End date filter (YYYY-MM-DD) | - |

#### List Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--include-hidden` | Include hidden assets in listing | `false` |
| `--include-recently-deleted` | Include recently deleted assets in listing | `false` |
| `--types` | Filter by asset types | all |
| `--format` | Output format (table, json) | `table` |

## Verifying Release Binaries 🔏

This project uses [goreleaser](https://goreleaser.com/) to build binaries and [actions/attest-build-provenance](https://github.com/actions/attest-build-provenance) to publish the provenance of the release.

You can verify the release binaries by following these steps:

1. Download a release from the [releases page](https://github.com/grantbirki/gh-photos/releases).
2. Verify it `gh attestation verify --owner grantbirki ~/Downloads/darwin-arm64` (an example for darwin-arm64).

---

Run `gh photos --help` for more information and full command/options usage.
