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
- Organized upload structure (photos/YYYY/MM/DD/category/)
- Dry-run mode for planning
- Parallel uploads with rClone
- Manifest generation for auditing
- Comprehensive filtering options`,
		Version: version.String(),
	}

	// Global flags
	cmd.PersistentFlags().Bool("no-color", false, "disable colored output")
	cmd.PersistentFlags().Bool("verbose", false, "enable verbose logging")
	cmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")

	// Add subcommands
	cmd.AddCommand(NewSyncCommand())
	cmd.AddCommand(NewValidateCommand())
	cmd.AddCommand(NewListCommand())

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
  gh photos sync /path/to/backup gdrive:Photos
  gh photos sync /backup/iphone s3:mybucket/photos --dry-run
  gh photos sync /backup gdrive:Photos --include-hidden --parallel 8`,
		Args: cobra.ExactArgs(2),
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
	cmd.Flags().BoolVar(&config.SkipExisting, "skip-existing", false, "skip files that already exist on remote")
	cmd.Flags().BoolVar(&config.Verify, "verify", false, "verify uploaded files match source")
	cmd.Flags().BoolVar(&config.ComputeChecksums, "checksum", false, "compute SHA256 checksums for assets")
	cmd.Flags().IntVar(&config.Parallel, "parallel", 4, "number of parallel uploads")
	cmd.Flags().StringVar(&config.RootPrefix, "root", "photos", "root directory prefix for uploads")
	cmd.Flags().StringVar(&config.SaveManifest, "save-manifest", "", "path to save operation manifest (JSON)")
	cmd.Flags().StringSliceVar(&config.AssetTypes, "types", nil, "comma-separated asset types to include (photos,videos,screenshots,burst,live_photos)")

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

		// Get global flags
		config.Verbose, _ = cmd.Flags().GetBool("verbose")
		config.LogLevel, _ = cmd.Flags().GetString("log-level")

		return nil
	}

	return cmd
}

// NewValidateCommand creates the validate subcommand
func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <backup-path>",
		Short: "Validate an iPhone backup directory",
		Long: `Validate checks if the specified directory contains a valid iPhone backup
with the required Photos.sqlite database and DCIM directory structure.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runValidate(args[0])
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
		Args: cobra.ExactArgs(1),
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

	// Try to find Photos.sqlite
	// This is a simplified validation - the real parser would do more thorough checks
	found := false
	filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, "Photos.sqlite") {
			color.Green("✓ Found Photos.sqlite at: %s", path)
			found = true
			return filepath.SkipDir
		}
		return nil
	})

	if !found {
		color.Yellow("⚠ Photos.sqlite not found - photo parsing may fail")
	}

	fmt.Println("Backup validation completed successfully")
	return nil
}

// runList lists assets in a backup
func runList(backupPath string) error {
	fmt.Printf("Listing assets in backup: %s\n", backupPath)

	// This would use the backup parser to list assets
	// For now, just validate the backup exists
	return runValidate(backupPath)
}
