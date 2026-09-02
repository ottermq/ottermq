package persistdb

import (
	"database/sql"

	"github.com/rs/zerolog/log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// const dbPath = "./data/ottermq.db"
var dbPath string

func SetDbPath(path string) {
	dbPath = path
}

func InitDB() {
	createTables()
}

func createTables() {
	createUserTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		role_id INTEGER NOT NULL,
		FOREIGN KEY(role_id) REFERENCES roles(id)
	);`
	_, err := db.Exec(createUserTable)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create 'users' table")
	}

	createRolesTable := `
	CREATE TABLE IF NOT EXISTS roles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		description TEXT
	);`
	_, err = db.Exec(createRolesTable)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create 'roles' table")
	}

	createPermissionsTable := `
	CREATE TABLE IF NOT EXISTS permissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		action TEXT UNIQUE NOT NULL,
		resource TEXT NOT NULL
	);`
	_, err = db.Exec(createPermissionsTable)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create 'permissions' table")
	}

	createRolePermissionsTable := `
	CREATE TABLE IF NOT EXISTS role_permissions (
		role_id INTEGER NOT NULL,
		permission_id INTEGER NOT NULL,
		FOREIGN KEY(role_id) REFERENCES roles(id),
		FOREIGN KEY(permission_id) REFERENCES permissions(id),
		PRIMARY KEY(role_id, permission_id)
	);`
	_, err = db.Exec(createRolePermissionsTable)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create 'role_permissions' table")
	}

	createUserVHostsTable := `
	CREATE TABLE IF NOT EXISTS user_vhosts (
		username TEXT NOT NULL,
		vhost TEXT NOT NULL,
		PRIMARY KEY(username, vhost),
		FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
	);`
	_, err = db.Exec(createUserVHostsTable)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create 'user_vhosts' table")
	}
}

func OpenDB() error {
	var err error
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Error().Err(err).Msg("Error opening database")
		return err
	}
	db.SetMaxOpenConns(1)
	return nil
}

func CloseDB() {
	if db != nil {
		db.Close()
	}
}
