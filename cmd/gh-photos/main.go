package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/grantbirki/gh-photos/internal/audit"
	"github.com/grantbirki/gh-photos/internal/backup"
	"github.com/grantbirki/gh-photos/internal/logger"
	"github.com/grantbirki/gh-photos/internal/photos"
	"github.com/grantbirki/gh-photos/internal/uploader"
	"github.com/grantbirki/gh-photos/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	if err := NewRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// NewRootCommand creates the root command
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh-photos",
		Short: "A GitHub CLI extension for backing up iPhone photos to cloud storage",
		Long: `gh-photos is a cross-platform CLI tool that ingests unencrypted iPhone backup folders,
parses the Photos database to identify asset types, and uploads them to rClone remotes
in an organized folder structure.

The tool supports:
- Asset classification (photos, videos, screenshots, burst, Live Photos)
- Privacy controls (excludes Hidden/Recently Deleted by default)  
- Organized upload structure (photos/YYYY/MM/DD/<category>/)
- Dry-run mode for planning
- Parallel uploads with rClone
- Manifest generation for auditing
- Comprehensive filtering options`,
		Version:      version.String(),
		SilenceUsage: true,
	}

	// Global flags
	cmd.PersistentFlags().Bool("no-color", false, "disable colored output")
	cmd.PersistentFlags().Bool("verbose", false, "enable verbose logging")
	cmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")

	// Add subcommands
	cmd.AddCommand(NewSyncCommand())
	cmd.AddCommand(NewValidateCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewExtractCommand())

	return cmd
}

// NewSyncCommand creates the sync subcommand
func NewSyncCommand() *cobra.Command {
	var config uploader.Config

	cmd := &cobra.Command{
		Use:   "sync <backup-path> <remote>",
		Short: "Sync iPhone photos from backup to remote storage",
		Long: `Sync extracts photos from an iPhone backup directory and uploads them to 
an rClone remote in an organized folder structure.

Examples:
  gh photos sync /path/to/backup gdrive:photos/backup/path
  gh photos sync /backup/iphone s3:mybucket/photos --dry-run
  gh photos sync /backup gdrive:photos --include-hidden --parallel 8`,
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Disable colors if requested
			if noColor, _ := cmd.Flags().GetBool("no-color"); noColor {
				color.NoColor = true
			}

			// Set config from args
			config.BackupPath = args[0]
			config.Remote = args[1]

			// Set root prefix (default to "photos")
			if config.RootPrefix == "" {
				config.RootPrefix = "photos"
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context(), config)
		},
	}

	// Sync-specific flags
	cmd.Flags().BoolVar(&config.IncludeHidden, "include-hidden", false, "include assets flagged as hidden")
	cmd.Flags().BoolVar(&config.IncludeRecentlyDeleted, "include-recently-deleted", false, "include assets flagged as recently deleted")
	cmd.Flags().BoolVar(&config.DryRun, "dry-run", false, "preview operations without uploading")
	cmd.Flags().BoolVar(&config.SkipExisting, "skip-existing", true, "skip files that already exist on remote")
	var forceOverwrite bool
	cmd.Flags().BoolVar(&forceOverwrite, "force-overwrite", false, "overwrite existing files on remote (opposite of --skip-existing)")
	cmd.Flags().BoolVar(&config.Verify, "verify", false, "verify uploaded files match source")
	cmd.Flags().BoolVar(&config.ComputeChecksums, "checksum", false, "compute SHA256 checksums for assets")
	cmd.Flags().IntVar(&config.Parallel, "parallel", 4, "number of parallel uploads")
	cmd.Flags().StringVar(&config.RootPrefix, "root", "photos", "root directory prefix for uploads")
	cmd.Flags().StringVar(&config.SaveManifest, "save-manifest", "", "path to save operation manifest (JSON)")
	cmd.Flags().StringSliceVar(&config.AssetTypes, "types", nil, "comma-separated asset types to include (photos,videos,screenshots,burst,live_photos)")

	// Audit trail flags
	cmd.Flags().StringVar(&config.SaveAuditManifest, "save-audit-manifest", "", "path to save an additional copy of the audit trail manifest (JSON)")
	cmd.Flags().BoolVar(&config.UseLastCommand, "use-last-command", false, "re-run the last successful command from ~/gh-photos/manifest.json")

	// Date filter flags
	var startDateStr, endDateStr string
	cmd.Flags().StringVar(&startDateStr, "start-date", "", "start date filter (YYYY-MM-DD)")
	cmd.Flags().StringVar(&endDateStr, "end-date", "", "end date filter (YYYY-MM-DD)")

	// Custom PreRunE to handle date parsing
	originalPreRunE := cmd.PreRunE
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if originalPreRunE != nil {
			if err := originalPreRunE(cmd, args); err != nil {
				return err
			}
		}

		// Parse date filters
		if startDateStr != "" {
			startDate, err := time.Parse("2006-01-02", startDateStr)
			if err != nil {
				return fmt.Errorf("invalid start-date format: %w", err)
			}
			config.StartDate = &startDate
		}

		if endDateStr != "" {
			endDate, err := time.Parse("2006-01-02", endDateStr)
			if err != nil {
				return fmt.Errorf("invalid end-date format: %w", err)
			}
			config.EndDate = &endDate
		}

		// Handle --use-last-command flag
		if config.UseLastCommand {
			if err := loadLastCommandConfig(&config, cmd, args); err != nil {
				return fmt.Errorf("failed to load last command: %w", err)
			}
		}

		// Handle force-overwrite flag (overrides skip-existing)
		if forceOverwrite {
			config.SkipExisting = false
		}

		// Get global flags
		config.Verbose, _ = cmd.Flags().GetBool("verbose")

		// Handle log level with environment variable fallback and case-insensitive matching
		logLevel, _ := cmd.Flags().GetString("log-level")

		// Check for LOG_LEVEL environment variable if flag wasn't explicitly set
		if !cmd.Flags().Changed("log-level") {
			if envLogLevel := os.Getenv("LOG_LEVEL"); envLogLevel != "" {
				logLevel = envLogLevel
			}
		}

		// Normalize log level to lowercase for case-insensitive matching
		config.LogLevel = strings.ToLower(strings.TrimSpace(logLevel))

		// Validate log level
		validLogLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if !validLogLevels[config.LogLevel] {
			return fmt.Errorf("invalid log level '%s'. Valid levels: debug, info, warn, error", config.LogLevel)
		}

		return nil
	}

	return cmd
}

// NewValidateCommand creates the validate subcommand
func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [backup-path]",
		Short: "Validate an iPhone backup directory",
		Long: `Validate checks if the specified directory contains a valid iPhone backup
with the required Photos.sqlite database and DCIM directory structure.
If no path is provided, checks the current working directory.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var backupPath string
			if len(args) == 0 {
				var err error
				backupPath, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get current working directory: %w", err)
				}
			} else {
				backupPath = args[0]
			}
			return runValidate(backupPath)
		},
	}

	return cmd
}

// NewListCommand creates the list subcommand
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <backup-path>",
		Short: "List assets found in an iPhone backup",
		Long: `List parses the iPhone backup and displays information about found assets
including their classification, flags, and file locations.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(args[0])
		},
	}

	cmd.Flags().Bool("include-hidden", false, "include hidden assets in listing")
	cmd.Flags().Bool("include-recently-deleted", false, "include recently deleted assets in listing")
	cmd.Flags().StringSlice("types", nil, "filter by asset types")
	cmd.Flags().String("format", "table", "output format (table, json)")

	return cmd
}

// NewExtractCommand creates the extract subcommand
func NewExtractCommand() *cobra.Command {
	var (
		outputPath   string
		skipExisting bool
		verify       bool
		progress     bool
	)

	cmd := &cobra.Command{
		Use:   "extract <backup-path> [output-path]",
		Short: "Extract unencrypted iTunes/Finder backup to readable directory structure",
		Long: `Extract reconstructs the original directory structure from an unencrypted iTunes or Finder backup.

This command reads the Manifest.db file to map hashed backup files back to their original
paths and domains, creating a readable directory structure similar to the original device.

Only unencrypted backups are supported. Encrypted backups will be rejected with an error.

Examples:
  gh photos extract /path/to/backup
  gh photos extract /backup/iPhone ./extracted --skip-existing
  gh photos extract /backup ./extracted --verify --progress`,
		Args:         cobra.RangeArgs(1, 2),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			backupPath := args[0]

			// Use provided output path or default
			if len(args) > 1 {
				outputPath = args[1]
			}
			if outputPath == "" {
				outputPath = "./extracted-backup"
			}

			return runExtract(backupPath, outputPath, skipExisting, verify, progress)
		},
	}

	// Extract-specific flags
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "output directory (default: ./extracted-backup)")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "skip files that already exist in output directory")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify extracted files by comparing checksums (disabled by default as it significantly slows extraction)")
	cmd.Flags().BoolVar(&progress, "progress", true, "show extraction progress")

	return cmd
}

// runExtract executes the backup extraction
func runExtract(backupPath, outputPath string, skipExisting, verify, progress bool) error {
	// Create logger
	loggerConfig := logger.Config{
		Level:  logger.LevelInfo,
		Output: os.Stdout,
	}
	log := logger.New(loggerConfig)

	log.Info("Starting iTunes backup extraction",
		"backup_path", backupPath,
		"output_path", outputPath)

	// Create extractor
	extractConfig := backup.ExtractConfig{
		BackupPath:   backupPath,
		OutputPath:   outputPath,
		SkipExisting: skipExisting,
		Verify:       verify,
		Progress:     progress,
		Logger:       log,
	}

	extractor, err := backup.NewExtractor(extractConfig)
	if err != nil {
		return fmt.Errorf("failed to create extractor: %w", err)
	}
	defer extractor.Close()

	// Perform extraction
	summary, err := extractor.Extract()
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Display summary
	color.Green("\n✓ Backup extraction completed successfully!")
	fmt.Printf("\nExtraction Summary:\n")
	fmt.Printf("  Total files processed: %d\n", summary.TotalFiles)
	fmt.Printf("  Files extracted: %d\n", summary.ExtractedFiles)
	fmt.Printf("  Files skipped: %d\n", summary.SkippedFiles)
	fmt.Printf("  Files failed: %d\n", summary.FailedFiles)
	fmt.Printf("  Domains found: %d\n", summary.DomainsFound)
	fmt.Printf("  Total size: %s\n", formatBytes(summary.TotalSize))
	fmt.Printf("  Extracted size: %s\n", formatBytes(summary.ExtractedSize))
	fmt.Printf("  Duration: %v\n", summary.Duration.Round(time.Second))

	if len(summary.Errors) > 0 {
		color.Yellow("\nWarnings/Errors:")
		for _, errMsg := range summary.Errors {
			fmt.Printf("  - %s\n", errMsg)
		}
	}

	fmt.Printf("\nExtracted backup is available at: %s\n", outputPath)
	return nil
}

// formatBytes formats bytes in human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// runSync executes the sync operation
func runSync(ctx context.Context, config uploader.Config) error {
	// Setup context cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Handle interrupt signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal, shutting down gracefully...")
		cancel()
	}()

	// Validate backup path
	if !filepath.IsAbs(config.BackupPath) {
		abs, err := filepath.Abs(config.BackupPath)
		if err != nil {
			return fmt.Errorf("failed to resolve backup path: %w", err)
		}
		config.BackupPath = abs
	}

	// Create uploader
	ul, err := uploader.NewUploader(config)
	if err != nil {
		return fmt.Errorf("failed to create uploader: %w", err)
	}
	defer ul.Close()

	// Execute sync
	return ul.Execute(ctx)
}

// runValidate validates a backup directory
func runValidate(backupPath string) error {
	fmt.Printf("Validating backup directory: %s\n", backupPath)
	fmt.Println()

	// Basic directory validation
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("backup path does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("backup path is not a directory")
	}

	// Check for Manifest.plist
	manifestPath := filepath.Join(backupPath, "Manifest.plist")
	if _, err := os.Stat(manifestPath); err != nil {
		return fmt.Errorf("Manifest.plist not found - this doesn't appear to be an iPhone backup")
	}

	color.Green("✓ Valid iPhone backup directory structure")

	// Check for Manifest.db (hashed backup structure)
	manifestDBPath := filepath.Join(backupPath, "Manifest.db")
	hasManifestDB := false
	if _, err := os.Stat(manifestDBPath); err == nil {
		hasManifestDB = true
		color.Green("✓ Found Manifest.db - this is a hashed iPhone backup")
	} else {
		color.Yellow("⚠ No Manifest.db found - checking for traditional directory structure")
	}

	// Try to find Photos.sqlite using the proper method
	fmt.Println("\nSearching for Photos.sqlite...")

	if hasManifestDB {
		// Use manifest database to find Photos.sqlite
		if err := validateWithManifestDB(backupPath); err != nil {
			color.Red("✗ Error using Manifest.db: %v", err)
			color.Yellow("⚠ Falling back to traditional search...")
			if err := validateTraditionalSearch(backupPath); err != nil {
				color.Red("✗ Traditional search also failed: %v", err)
				return fmt.Errorf("could not locate Photos.sqlite")
			}
		}
	} else {
		// Use traditional directory search
		if err := validateTraditionalSearch(backupPath); err != nil {
			color.Red("✗ Photos.sqlite not found: %v", err)
			return fmt.Errorf("could not locate Photos.sqlite")
		}
	}

	fmt.Println()
	color.Green("✓ Backup validation completed successfully")
	return nil
}

// validateWithManifestDB validates using Manifest.db
func validateWithManifestDB(backupPath string) error {
	manifestDB, err := backup.OpenManifestDB(backupPath)
	if err != nil {
		return err
	}
	defer manifestDB.Close()

	// Show available domains for debugging
	domains, err := manifestDB.GetDomains()
	if err != nil {
		color.Yellow("⚠ Could not list domains: %v", err)
	} else {
		fmt.Printf("Found %d domains in backup\n", len(domains))

		// Show photo-related domains
		photoRelatedDomains := []string{}
		for _, domain := range domains {
			domainLower := strings.ToLower(domain)
			if strings.Contains(domainLower, "photo") ||
				strings.Contains(domainLower, "media") ||
				strings.Contains(domainLower, "camera") {
				photoRelatedDomains = append(photoRelatedDomains, domain)
			}
		}

		if len(photoRelatedDomains) > 0 {
			color.Cyan("📸 Photo-related domains found:")
			for _, domain := range photoRelatedDomains {
				fmt.Printf("  - %s\n", domain)
			}
		}
	}

	// List all photos-related files for detailed information
	photosFiles, err := manifestDB.ListPhotosRelatedFiles()
	if err != nil {
		color.Yellow("⚠ Could not list photos-related files: %v", err)
	} else if len(photosFiles) > 0 {
		color.Cyan("\n📋 Photos-related files found:")
		for _, file := range photosFiles {
			fmt.Printf("  - %s: %s\n", file.Domain, file.RelativePath)
		}
	}

	// Try to find Photos.sqlite specifically
	photosPath, err := manifestDB.FindPhotosDatabase(backupPath)
	if err != nil {
		return fmt.Errorf("Photos.sqlite not found in Manifest.db: %w", err)
	}

	// Validate the database
	if err := photos.ValidateDatabase(photosPath); err != nil {
		return fmt.Errorf("found Photos.sqlite but validation failed: %w", err)
	}

	color.Green("✓ Found and validated Photos.sqlite at: %s", photosPath)
	return nil
}

// validateTraditionalSearch validates using traditional directory search
func validateTraditionalSearch(backupPath string) error {
	found := false
	var foundPath string

	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}

		if strings.HasSuffix(path, "Photos.sqlite") {
			// Validate it's actually a Photos database
			if err := photos.ValidateDatabase(path); err == nil {
				foundPath = path
				found = true
				return filepath.SkipDir
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error searching for Photos.sqlite: %w", err)
	}

	if !found {
		return fmt.Errorf("Photos.sqlite not found in backup directory")
	}

	color.Green("✓ Found and validated Photos.sqlite at: %s", foundPath)
	return nil
}

// runList lists assets in a backup
func runList(backupPath string) error {
	fmt.Printf("Listing assets in backup: %s\n", backupPath)

	// This would use the backup parser to list assets
	// For now, just validate the backup exists
	return runValidate(backupPath)
}

// loadLastCommandConfig loads configuration from the last successful run
func loadLastCommandConfig(config *uploader.Config, cmd *cobra.Command, args []string) error {
	// Load the last manifest
	trail, err := audit.LoadLatestManifest()
	if err != nil {
		return fmt.Errorf("could not load last manifest from ~/gh-photos/manifest.json: %w", err)
	}

	// Override config with values from manifest, but allow command line flags to take precedence
	if !cmd.Flags().Changed("include-hidden") {
		config.IncludeHidden = trail.Metadata.Invocation.Flags.IncludeHidden
	}
	if !cmd.Flags().Changed("include-recently-deleted") {
		config.IncludeRecentlyDeleted = trail.Metadata.Invocation.Flags.IncludeRecentlyDeleted
	}
	if !cmd.Flags().Changed("parallel") {
		config.Parallel = trail.Metadata.Invocation.Flags.Parallel
	}
	if !cmd.Flags().Changed("skip-existing") {
		config.SkipExisting = trail.Metadata.Invocation.Flags.SkipExisting
	}
	if !cmd.Flags().Changed("dry-run") {
		config.DryRun = trail.Metadata.Invocation.Flags.DryRun
	}
	if !cmd.Flags().Changed("log-level") {
		config.LogLevel = trail.Metadata.Invocation.Flags.LogLevel
	}
	if !cmd.Flags().Changed("types") && len(trail.Metadata.Invocation.Flags.Types) > 0 {
		config.AssetTypes = trail.Metadata.Invocation.Flags.Types
	}
	if !cmd.Flags().Changed("start-date") && trail.Metadata.Invocation.Flags.StartDate != nil {
		config.StartDate = trail.Metadata.Invocation.Flags.StartDate
	}
	if !cmd.Flags().Changed("end-date") && trail.Metadata.Invocation.Flags.EndDate != nil {
		config.EndDate = trail.Metadata.Invocation.Flags.EndDate
	}
	if !cmd.Flags().Changed("root") && trail.Metadata.Invocation.Flags.Root != "" {
		config.RootPrefix = trail.Metadata.Invocation.Flags.Root
	}
	if !cmd.Flags().Changed("verify") {
		config.Verify = trail.Metadata.Invocation.Flags.Verify
	}
	if !cmd.Flags().Changed("checksum") {
		config.ComputeChecksums = trail.Metadata.Invocation.Flags.Checksum
	}

	// Override backup path and remote if not provided as arguments
	if len(args) == 0 {
		config.BackupPath = trail.Metadata.Device.BackupPath
		config.Remote = trail.Metadata.Invocation.Remote
	} else if len(args) == 1 {
		// Only backup path provided, use remote from manifest
		config.Remote = trail.Metadata.Invocation.Remote
	}
	// If both args provided, use them (already set in main PreRunE)

	fmt.Printf("✓ Loaded configuration from last successful run (%s)\n", trail.Metadata.RunID)
	return nil
}
