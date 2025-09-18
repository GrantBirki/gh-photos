# gh-photos 📸

[![test](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/test.yml)
[![build](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/build.yml)
[![lint](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/lint.yml)
[![release](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml/badge.svg)](https://github.com/grantbirki/gh-photos/actions/workflows/release.yml)
![slsa-level3](docs/assets/slsa-level3.svg)

A gh cli extension for uploading iPhone photos from a backup dump to a remote target using rclone and logical defaults.

![logo](docs/assets/logo.png)

## About ⭐

This project is a [`gh cli`](https://github.com/cli/cli) extension that extracts photos and videos from unencrypted iPhone backup directories and uploads them to cloud storage using [rclone](https://rclone.org/). The tool intelligently parses the `Photos.sqlite` database to classify assets by type and organizes them into a clean folder structure on your chosen cloud provider.

**Key Features:**

- 📱 Extracts photos from unencrypted iPhone backups
- 🗂️ Organizes uploads by date and asset type (`photos/YYYY/MM/DD/<category>/`)
- 🔐 Privacy-safe defaults (excludes Hidden/Recently Deleted albums)
- ☁️ Supports all **rclone** remotes (Google Drive, S3, OneDrive, etc.)
- ⚡ Parallel uploads for faster performance
- � Smart defaults (skips existing files to save bandwidth)
- �🔍 Asset classification (photos, videos, screenshots, burst, Live Photos)
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

## Finding iPhone Backup Paths 📱

The CLI automatically walks directory structures to find your iPhone backup. You can provide any of these common parent directories, and the tool will locate the actual backup folder:

### Windows Backup Locations

```bash
# Common Windows iPhone backup paths (the CLI will auto-navigate to the backup folder)
C:\Users\<username>\Apple\MobileSync\
C:\Users\<username>\Apple\MobileSync\Backup\
C:\Users\<username>\AppData\Roaming\Apple Computer\MobileSync\
C:\Users\<username>\AppData\Roaming\Apple Computer\MobileSync\Backup\
```

### macOS Backup Locations

```bash
# Common macOS iPhone backup paths (the CLI will auto-navigate to the backup folder)
~/Library/Application Support/MobileSync/
~/Library/Application Support/MobileSync/Backup/
/Users/<username>/Library/Application Support/MobileSync/
/Users/<username>/Library/Application Support/MobileSync/Backup/
```

**Smart Path Resolution**: The CLI automatically detects if you've provided a parent directory and will:

1. Look for a `Backup` subdirectory
2. Check if that contains `Manifest.db` or `Manifest.plist` files
3. If there's a single backup folder inside, navigate into it
4. Validate that it's a proper iPhone backup directory

This means you can simply point to `/Users/username/Library/Application Support/MobileSync/` and the tool will find the actual backup directory automatically.

## iTunes Backup Extraction 🗂️

The `extract` command allows you to extract unencrypted iTunes/Finder backups into a readable directory structure before processing. This is useful when your backup files are in the hashed format that iTunes uses internally.

### Why Extract?

iTunes and Finder create backups with hashed filenames (like `00/1a2b3c4e5f...`) instead of the original file names. The extract command:

- ✅ **Reconstructs original paths** using the backup's `Manifest.db`
- ✅ **Organizes by domain** (MediaDomain, HomeDomain, etc.)
- ✅ **Only supports unencrypted backups** (encrypted backups are rejected for security)
- ✅ **Shows progress and provides detailed summary**
- ✅ **Optionally verifies file integrity** with checksums

### Extract Examples

```bash
# Basic extraction (creates ./extracted-backup/)
gh photos extract /path/to/backup

# Extract to specific directory
gh photos extract /path/to/backup ./my-extracted-backup

# Extract with verification and progress
gh photos extract /backup ./extracted --verify --progress

# Skip files that already exist
gh photos extract /backup ./extracted --skip-existing
```

After extraction, you can run normal sync operations:

```bash
# Extract first, then sync
gh photos extract /path/to/backup ./extracted
gh photos sync ./extracted/MediaDomain gdrive:photos
```

## Usage 🚀

The `gh-photos` extension provides three main commands for working with iPhone backup photos:

### Basic Commands

```bash
# Extract iTunes/Finder backup to readable directory structure
gh photos extract /path/to/backup
gh photos extract /path/to/backup ./extracted-backup

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
# Sync with custom settings (skips existing files by default)
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

# Upload to custom root directory (skipping existing files by default)
gh photos sync /backup gdrive:photos \
  --root "family-photos" \
  --verify

# Force overwrite existing files
gh photos sync /backup gdrive:photos \
  --force-overwrite \
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
| `--skip-existing` | Skip files that already exist on remote (smart default) | `true` |
| `--force-overwrite` | Overwrite existing files on remote (opposite of --skip-existing) | `false` |
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

#### Extract Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output` | Output directory for extracted files | `./extracted-backup` |
| `--skip-existing` | Skip files that already exist in output directory | `false` |
| `--verify` | Verify extracted files by comparing checksums (significantly slows extraction) | `false` |
| `--progress` | Show extraction progress during operation | `true` |

## Verifying Release Binaries 🔏

This project uses [goreleaser](https://goreleaser.com/) to build binaries and [actions/attest-build-provenance](https://github.com/actions/attest-build-provenance) to publish the provenance of the release.

You can verify the release binaries by following these steps:

1. Download a release from the [releases page](https://github.com/grantbirki/gh-photos/releases).
2. Verify it `gh attestation verify --owner grantbirki ~/Downloads/darwin-arm64` (an example for darwin-arm64).

---

Run `gh photos --help` for more information and full command/options usage.
