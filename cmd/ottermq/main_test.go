package main

import (
	"path/filepath"
	"testing"

	"github.com/ottermq/ottermq/config"
	"github.com/ottermq/ottermq/internal/persistdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupUserDatabase_BootstrapsMissingUserInExistingDB(t *testing.T) {
	defer persistdb.CloseDB()

	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "ottermq.db")

	persistdb.SetDbPath(dbPath)
	require.NoError(t, persistdb.OpenDB())
	persistdb.InitDB()
	persistdb.CloseDB()

	cfg := &config.Config{
		Username: "guest",
		Password: "guest",
	}

	user, err := setupUserDatabase(dataDir, cfg)
	require.NoError(t, err)
	assert.Equal(t, "guest", user.Username)
	assert.Equal(t, 1, user.RoleID)

	hasAccess, err := persistdb.HasVHostAccess("guest", "/")
	require.NoError(t, err)
	assert.True(t, hasAccess)
}
