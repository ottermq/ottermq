package persistdb

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	bcryptCost = bcrypt.MinCost
	dir := t.TempDir()
	SetDbPath(filepath.Join(dir, "test.db"))
	require.NoError(t, OpenDB())
	InitDB()
	AddDefaultRoles()
	AddDefaultPermissions()
	return func() {
		CloseDB()
		bcryptCost = 14
	}
}

func TestOpenAndClose(t *testing.T) {
	dir := t.TempDir()
	SetDbPath(filepath.Join(dir, "test.db"))
	require.NoError(t, OpenDB())
	CloseDB()
}

func TestInitDB_CreatesTables(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	// If tables were not created, queries below would fail.
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	require.NoError(t, err)
	defer rows.Close()

	tables := map[string]bool{}
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables[name] = true
	}
	assert.True(t, tables["users"])
	assert.True(t, tables["roles"])
	assert.True(t, tables["permissions"])
	assert.True(t, tables["role_permissions"])
}

func TestAddUser_And_GetUserByUsername(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	err := AddUser(UserCreateDTO{Username: "alice", Password: "password123", RoleID: 1})
	require.NoError(t, err)

	user, err := GetUserByUsername("alice")
	require.NoError(t, err)
	assert.Equal(t, "alice", user.Username)
	assert.Equal(t, 1, user.RoleID)
}

func TestGetUsers(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	require.NoError(t, AddUser(UserCreateDTO{Username: "u1", Password: "pass", RoleID: 1}))
	require.NoError(t, AddUser(UserCreateDTO{Username: "u2", Password: "pass", RoleID: 1}))

	users, err := GetUsers()
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestAuthenticateUser(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	require.NoError(t, AddUser(UserCreateDTO{Username: "bob", Password: "secret", RoleID: 1}))

	ok, err := AuthenticateUser("bob", "secret")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = AuthenticateUser("bob", "wrong")
	require.NoError(t, err)
	assert.False(t, ok)

	ok, err = AuthenticateUser("unknown", "pass")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestGetRoleByID(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	role, err := GetRoleByID(1)
	require.NoError(t, err)
	assert.Equal(t, "admin", role.Name)
}

func TestToUserListDTO(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	require.NoError(t, AddUser(UserCreateDTO{Username: "carol", Password: "pass", RoleID: 1}))
	user, err := GetUserByUsername("carol")
	require.NoError(t, err)

	dto, err := user.ToUserListDTO()
	require.NoError(t, err)
	assert.Equal(t, "carol", dto.Username)
	assert.Equal(t, "admin", dto.Role)
}

// TestConcurrentReads verifies that multiple goroutines can read from the DB
// simultaneously without data races (the fix for issue #2).
func TestConcurrentReads(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	require.NoError(t, AddUser(UserCreateDTO{Username: "shared", Password: "pass", RoleID: 1}))

	var wg sync.WaitGroup
	const goroutines = 20
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = GetUserByUsername("shared")
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "goroutine %d failed", i)
	}
}

// TestConcurrentWrites verifies that multiple goroutines can write to the DB
// without corrupting state (the fix for issue #2).
func TestConcurrentWrites(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	var wg sync.WaitGroup
	const goroutines = 10
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = AddUser(UserCreateDTO{
				Username: filepath.Base(os.TempDir()) + "_user_" + string(rune('a'+idx)),
				Password: "pass",
				RoleID:   1,
			})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "goroutine %d failed", i)
	}

	users, err := GetUsers()
	require.NoError(t, err)
	assert.Len(t, users, goroutines)
}

// TestConcurrentMixedReadWrite verifies reads and writes don't race each other.
func TestConcurrentMixedReadWrite(t *testing.T) {
	teardown := setupTestDB(t)
	defer teardown()

	require.NoError(t, AddUser(UserCreateDTO{Username: "existing", Password: "pass", RoleID: 1}))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			GetUserByUsername("existing") //nolint:errcheck
		}()
		go func(idx int) {
			defer wg.Done()
			AddUser(UserCreateDTO{ //nolint:errcheck
				Username: "new_" + string(rune('a'+idx)),
				Password: "pass",
				RoleID:   1,
			})
		}(i)
	}
	wg.Wait()
}
