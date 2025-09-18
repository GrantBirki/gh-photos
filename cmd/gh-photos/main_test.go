package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()

	assert.Equal(t, "gh-photos", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.True(t, cmd.HasSubCommands())
}

func TestNewSyncCommand(t *testing.T) {
	cmd := NewSyncCommand()

	assert.Equal(t, "sync <backup-path> <remote>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.True(t, cmd.HasFlags())

	// Check that required flags exist
	assert.NotNil(t, cmd.Flags().Lookup("dry-run"))
	assert.NotNil(t, cmd.Flags().Lookup("include-hidden"))
	assert.NotNil(t, cmd.Flags().Lookup("parallel"))
}

func TestNewValidateCommand(t *testing.T) {
	cmd := NewValidateCommand()

	assert.Equal(t, "validate <backup-path>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
}

func TestNewListCommand(t *testing.T) {
	cmd := NewListCommand()

	assert.Equal(t, "list <backup-path>", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.True(t, cmd.HasFlags())
}
