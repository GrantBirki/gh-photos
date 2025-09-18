package uploader

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/grantbirki/gh-photos/internal/audit"
	"github.com/grantbirki/gh-photos/internal/backup"
	"github.com/grantbirki/gh-photos/internal/manifest"
	"github.com/grantbirki/gh-photos/internal/rclone"
	"github.com/grantbirki/gh-photos/internal/types"
	"github.com/grantbirki/gh-photos/internal/version"
)

// Config represents the configuration for the uploader
type Config struct {
	BackupPath             string
	Remote                 string
	RootPrefix             string
	IncludeHidden          bool
	IncludeRecentlyDeleted bool
	DryRun                 bool
	SkipExisting           bool
	Verify                 bool
	Parallel               int
	StartDate              *time.Time
	EndDate                *time.Time
	AssetTypes             []string
	SaveManifest           string
	ComputeChecksums       bool
	LogLevel               string
	Verbose                bool
	SaveAuditManifest      string
	UseLastCommand         bool
}

// Uploader orchestrates the photo backup process
type Uploader struct {
	config         Config
	logger         *log.Logger
	parser         *backup.BackupParser
	rcloneClient   *rclone.Client
	manifest       *manifest.Manifest
	auditTrail     *audit.TrailManager
	filteredAssets []*types.Asset // Store filtered assets for audit trail
}

// NewUploader creates a new uploader instance
func NewUploader(config Config) (*Uploader, error) {
	// Setup logger
	logger := log.New(os.Stdout, "", log.LstdFlags)

	// Set appropriate output for debug messages
	if config.LogLevel == "debug" || config.Verbose {
		logger.SetOutput(os.Stdout)
	} else {
		// For non-debug levels, we'll filter debug messages in the rclone client
		logger.SetOutput(os.Stdout)
	}

	// Validate rclone installation
	if !config.DryRun {
		if err := rclone.ValidateRcloneInstallation(); err != nil {
			return nil, fmt.Errorf("rclone validation failed: %w", err)
		}

		if err := rclone.ValidateRemote(config.Remote); err != nil {
			return nil, fmt.Errorf("remote validation failed: %w", err)
		}
	}

	// Create backup parser
	parser, err := backup.NewBackupParser(config.BackupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup parser: %w", err)
	}

	// Create rclone client
	rcloneClient := rclone.NewClient(
		config.Remote,
		config.Parallel,
		config.Verify,
		config.DryRun,
		config.SkipExisting,
		logger,
		config.LogLevel,
	)

	// Create audit trail manager
	auditTrail, err := audit.NewTrailManager(version.String())
	if err != nil {
		return nil, fmt.Errorf("failed to create audit trail manager: %w", err)
	}

	return &Uploader{
		config:       config,
		logger:       logger,
		parser:       parser,
		rcloneClient: rcloneClient,
		auditTrail:   auditTrail,
	}, nil
}

// Close cleans up resources
func (u *Uploader) Close() error {
	if u.parser != nil {
		return u.parser.Close()
	}
	return nil
}

// Execute runs the complete backup process
func (u *Uploader) Execute(ctx context.Context) error {
	startTime := time.Now()

	u.logInfo("Starting iPhone photo backup process...")
	u.logInfo("Backup path: %s", u.config.BackupPath)
	u.logInfo("Remote target: %s", u.config.Remote)

	// Setup audit trail
	if err := u.setupAuditTrail(); err != nil {
		return fmt.Errorf("failed to setup audit trail: %w", err)
	}

	// Parse assets from backup
	u.logInfo("Parsing assets from backup...")
	assets, err := u.parser.ParseAssets()
	if err != nil {
		return fmt.Errorf("failed to parse assets: %w", err)
	}
	u.logInfo("Found %d total assets", len(assets))

	// Filter assets
	u.filteredAssets = u.filterAssets(assets)
	u.logInfo("After filtering: %d assets to process", len(u.filteredAssets))

	if len(u.filteredAssets) == 0 {
		u.logInfo("No assets to process. Exiting.")
		return nil
	}

	// Compute checksums if requested
	if u.config.ComputeChecksums {
		u.logInfo("Computing checksums...")
		if err := u.computeChecksums(u.filteredAssets); err != nil {
			return fmt.Errorf("failed to compute checksums: %w", err)
		}
	}

	// Create manifest
	manifestConfig := manifest.Config{
		IncludeHidden:          u.config.IncludeHidden,
		IncludeRecentlyDeleted: u.config.IncludeRecentlyDeleted,
		DryRun:                 u.config.DryRun,
		SkipExisting:           u.config.SkipExisting,
		Verify:                 u.config.Verify,
		Parallel:               u.config.Parallel,
		StartDate:              u.config.StartDate,
		EndDate:                u.config.EndDate,
		AssetTypes:             u.config.AssetTypes,
	}

	generator := manifest.NewGenerator(u.config.BackupPath, u.config.Remote, manifestConfig)
	u.manifest = generator.CreateManifest(u.filteredAssets, u.config.RootPrefix)

	// Create upload plan
	u.logInfo("Creating upload plan...")
	plan, err := u.rcloneClient.CreateUploadPlan(ctx, u.manifest.Entries)
	if err != nil {
		return fmt.Errorf("failed to create upload plan: %w", err)
	}

	// Display plan
	if u.config.DryRun || u.config.Verbose {
		rclone.PrintUploadPlan(plan)
	}

	// Execute uploads if not dry run
	if !u.config.DryRun {
		u.logInfo("Starting uploads...")

		// Filter plan entries that need uploading
		var uploadEntries []manifest.Entry
		for _, planEntry := range plan {
			if planEntry.Action == rclone.ActionUpload {
				uploadEntries = append(uploadEntries, planEntry.Entry)
			} else if planEntry.Action == rclone.ActionSkip {
				// Update manifest status for skipped entries
				for i, entry := range u.manifest.Entries {
					if entry.SourcePath == planEntry.Entry.SourcePath {
						u.manifest.UpdateEntry(i, manifest.StatusSkipped, "")
						break
					}
				}
			}
		}

		// Execute uploads
		if len(uploadEntries) > 0 {
			err := u.rcloneClient.UploadBatch(ctx, uploadEntries, u.updateManifestCallback)
			if err != nil {
				return fmt.Errorf("upload failed: %w", err)
			}
		}

		// Verify uploads if requested
		if u.config.Verify {
			u.logInfo("Verifying uploads...")
			if err := u.verifyUploads(ctx); err != nil {
				u.logError("Verification failed: %v", err)
			}
		}
	}

	// Update manifest summary
	duration := time.Since(startTime)
	u.manifest.Summary.DurationSeconds = int64(duration.Seconds())

	// Save manifest if requested
	if u.config.SaveManifest != "" {
		u.logInfo("Saving manifest to %s", u.config.SaveManifest)
		if err := u.manifest.SaveToFile(u.config.SaveManifest); err != nil {
			u.logError("Failed to save manifest: %v", err)
		}
	}

	// Print final summary
	u.manifest.PrintSummary()

	// Finalize audit trail (only on successful completion)
	if err := u.finalizeAuditTrail(); err != nil {
		u.logError("Failed to finalize audit trail: %v", err)
		// Don't return error - the sync was successful
	}

	u.logInfo("Backup process completed in %v", duration)
	return nil
}

// filterAssets applies filters to the asset list
func (u *Uploader) filterAssets(assets []*types.Asset) []*types.Asset {
	var filtered []*types.Asset
	var hiddenCount, recentlyDeletedCount, dateFilteredCount, typeFilteredCount int

	for _, asset := range assets {
		// Apply exclusion rules and count what's being excluded
		if asset.ShouldExclude(u.config.IncludeHidden, u.config.IncludeRecentlyDeleted) {
			if asset.Flags.Hidden && !u.config.IncludeHidden {
				hiddenCount++
			}
			if asset.Flags.RecentlyDeleted && !u.config.IncludeRecentlyDeleted {
				recentlyDeletedCount++
			}
			continue
		}

		// Apply date filters
		if u.config.StartDate != nil && asset.CreationDate.Before(*u.config.StartDate) {
			dateFilteredCount++
			continue
		}
		if u.config.EndDate != nil && asset.CreationDate.After(*u.config.EndDate) {
			dateFilteredCount++
			continue
		}

		// Apply type filters
		if len(u.config.AssetTypes) > 0 {
			typeMatch := false
			for _, allowedType := range u.config.AssetTypes {
				if strings.EqualFold(string(asset.Type), allowedType) {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				typeFilteredCount++
				continue
			}
		}

		// Generate target path
		asset.TargetPath = asset.GenerateTargetPath(u.config.RootPrefix)

		filtered = append(filtered, asset)
	}

	// Log exclusion counts at info level for user visibility
	if hiddenCount > 0 {
		u.logInfo("Excluding %d hidden assets (use --include-hidden to include them)", hiddenCount)
	}
	if recentlyDeletedCount > 0 {
		u.logInfo("Excluding %d recently deleted assets (use --include-recently-deleted to include them)", recentlyDeletedCount)
	}
	if dateFilteredCount > 0 {
		u.logInfo("Excluding %d assets due to date filters", dateFilteredCount)
	}
	if typeFilteredCount > 0 {
		u.logInfo("Excluding %d assets due to type filters", typeFilteredCount)
	}

	return filtered
}

// computeChecksums calculates checksums for all assets
func (u *Uploader) computeChecksums(assets []*types.Asset) error {
	for i, asset := range assets {
		if u.config.Verbose {
			u.logInfo("Computing checksum for %s (%d/%d)",
				filepath.Base(asset.SourcePath), i+1, len(assets))
		}

		if err := asset.ComputeChecksum(); err != nil {
			u.logError("Failed to compute checksum for %s: %v", asset.SourcePath, err)
			continue
		}
	}
	return nil
}

// updateManifestCallback updates the manifest when an upload completes
func (u *Uploader) updateManifestCallback(index int, status manifest.OperationStatus, errorMsg string) {
	u.manifest.UpdateEntry(index, status, errorMsg)

	if u.config.Verbose {
		entry := u.manifest.Entries[index]
		if status == manifest.StatusUploaded {
			u.logSuccess("Uploaded: %s", filepath.Base(entry.SourcePath))
		} else if status == manifest.StatusFailed {
			u.logError("Failed: %s - %s", filepath.Base(entry.SourcePath), errorMsg)
		}
	}
}

// verifyUploads verifies that uploaded files match the source
func (u *Uploader) verifyUploads(ctx context.Context) error {
	uploadedEntries := u.manifest.GetFilteredEntries(manifest.StatusUploaded)

	for i, entry := range uploadedEntries {
		if u.config.Verbose {
			u.logInfo("Verifying %s (%d/%d)",
				filepath.Base(entry.SourcePath), i+1, len(uploadedEntries))
		}

		if err := u.rcloneClient.VerifyUpload(ctx, entry); err != nil {
			u.logError("Verification failed for %s: %v", entry.SourcePath, err)
			// Update manifest status
			for j, manifestEntry := range u.manifest.Entries {
				if manifestEntry.SourcePath == entry.SourcePath {
					u.manifest.UpdateEntry(j, manifest.StatusFailed, "verification failed")
					break
				}
			}
		} else {
			// Update manifest status to verified
			for j, manifestEntry := range u.manifest.Entries {
				if manifestEntry.SourcePath == entry.SourcePath {
					u.manifest.UpdateEntry(j, manifest.StatusVerified, "")
					break
				}
			}
		}
	}

	return nil
}

// Logging methods
func (u *Uploader) logInfo(format string, args ...interface{}) {
	if u.config.Verbose || u.config.LogLevel == "debug" || u.config.LogLevel == "info" {
		message := fmt.Sprintf(format, args...)
		u.logger.Printf("[INFO] %s", message)
	}
}

func (u *Uploader) logError(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	color.Red("[ERROR] " + message)
	u.logger.Printf("[ERROR] %s", message)
}

func (u *Uploader) logSuccess(format string, args ...interface{}) {
	if u.config.Verbose {
		message := fmt.Sprintf(format, args...)
		color.Green("[SUCCESS] " + message)
	}
}

// setupAuditTrail initializes the audit trail with device and invocation information
func (u *Uploader) setupAuditTrail() error {
	// Set device information (we might need to extract this from backup metadata)
	deviceName, deviceUUID, iosVersion := u.extractDeviceInfo()
	u.auditTrail.SetDeviceInfo(u.config.BackupPath, deviceName, deviceUUID, iosVersion)

	// Set invocation details
	flags := audit.InvocationFlags{
		IncludeHidden:          u.config.IncludeHidden,
		IncludeRecentlyDeleted: u.config.IncludeRecentlyDeleted,
		Parallel:               u.config.Parallel,
		SkipExisting:           u.config.SkipExisting,
		DryRun:                 u.config.DryRun,
		LogLevel:               u.config.LogLevel,
		Types:                  u.config.AssetTypes,
		StartDate:              u.config.StartDate,
		EndDate:                u.config.EndDate,
		Root:                   u.config.RootPrefix,
		Verify:                 u.config.Verify,
		Checksum:               u.config.ComputeChecksums,
	}

	u.auditTrail.SetInvocation(u.config.Remote, flags)
	return nil
}

// extractDeviceInfo extracts device information from the backup
func (u *Uploader) extractDeviceInfo() (*string, *string, *string) {
	// Try to extract device info from Info.plist if available
	// For now, return nil values - this can be enhanced later
	return nil, nil, nil
}

// finalizeAuditTrail completes the audit trail and saves it
func (u *Uploader) finalizeAuditTrail() error {
	// Add all assets from manifest to audit trail
	for _, entry := range u.manifest.Entries {
		// Find the original asset to get complete information
		asset := u.findAssetBySourcePath(entry.SourcePath)
		if asset != nil {
			status := u.manifestStatusToAuditStatus(entry.Status)
			u.auditTrail.AddAsset(asset, entry.TargetPath, status)
		}
	}

	// Finalize and save audit trail
	if err := u.auditTrail.Finalize(); err != nil {
		return err
	}

	// Save additional copy if requested
	if u.config.SaveAuditManifest != "" {
		if err := u.auditTrail.SaveAdditionalCopy(u.config.SaveAuditManifest); err != nil {
			u.logError("Failed to save additional audit manifest copy: %v", err)
		} else {
			u.logInfo("Saved additional audit manifest to: %s", u.config.SaveAuditManifest)
		}
	}

	u.logInfo("✓ Audit trail saved to ~/gh-photos/")
	return nil
}

// findAssetBySourcePath finds an asset by its source path (helper for audit trail)
func (u *Uploader) findAssetBySourcePath(sourcePath string) *types.Asset {
	for _, asset := range u.filteredAssets {
		if asset.SourcePath == sourcePath {
			return asset
		}
	}
	return nil
}

// manifestStatusToAuditStatus converts manifest status to audit trail status
func (u *Uploader) manifestStatusToAuditStatus(status manifest.OperationStatus) string {
	switch status {
	case manifest.StatusUploaded:
		return "uploaded"
	case manifest.StatusSkipped:
		return "skipped"
	case manifest.StatusFailed:
		return "failed"
	default:
		return "skipped"
	}
}
