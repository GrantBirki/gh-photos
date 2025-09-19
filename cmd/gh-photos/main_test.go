package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRootCommand(t *testing.T) {
	cmd := CreateRootCommand()

	assert.Equal(t, "gh-photos", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.True(t, cmd.HasSubCommands())
}

func TestNewSyncCommand(t *testing.T) {
	cmd := CreateSyncCommand()

	assert.Equal(t, "sync <backup-path> <remote>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.True(t, cmd.HasFlags())

	// Check that required flags exist
	assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
	assert.NotNil(t, cmd.Flags().Lookup("include-hidden"))
	assert.NotNil(t, cmd.Flags().Lookup("parallel"))
	assert.NotNil(t, cmd.Flags().Lookup("skip-existing"))
	assert.NotNil(t, cmd.Flags().Lookup("force-overwrite"))

	// Test default values
	skipExistingFlag := cmd.Flags().Lookup("skip-existing")
	assert.Equal(t, "true", skipExistingFlag.DefValue, "skip-existing should default to true")

	forceOverwriteFlag := cmd.Flags().Lookup("force-overwrite")
	assert.Equal(t, "false", forceOverwriteFlag.DefValue, "force-overwrite should default to false")
}

func TestNewValidateCommand(t *testing.T) {
	cmd := CreateValidateCommand()

	assert.Equal(t, "validate [backup-path]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestNewListCommand(t *testing.T) {
	cmd := CreateListCommand()

	assert.Equal(t, "list <backup-path>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.True(t, cmd.HasFlags())
}

func TestRootCommandLogLevel(t *testing.T) {
	cmd := CreateRootCommand()

	// Check that log-level flag exists and has correct default
	logLevelFlag := cmd.PersistentFlags().Lookup("log-level")
	assert.NotNil(t, logLevelFlag)
	assert.Equal(t, "info", logLevelFlag.DefValue, "log-level should default to info")
}
