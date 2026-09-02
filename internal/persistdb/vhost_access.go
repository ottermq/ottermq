package persistdb

import (
	"database/sql"
	"fmt"

	"github.com/rs/zerolog/log"
)

// GrantVHostAccess gives a user access to a vhost.
func GrantVHostAccess(username, vhost string) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO user_vhosts (username, vhost) VALUES (?, ?)",
		username, vhost,
	)
	if err != nil {
		log.Error().Err(err).Str("username", username).Str("vhost", vhost).Msg("Failed to grant vhost access")
	}
	return err
}

// RevokeVHostAccess removes a user's access to a vhost.
func RevokeVHostAccess(username, vhost string) error {
	result, err := db.Exec(
		"DELETE FROM user_vhosts WHERE username = ? AND vhost = ?",
		username, vhost,
	)
	if err != nil {
		log.Error().Err(err).Str("username", username).Str("vhost", vhost).Msg("Failed to revoke vhost access")
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no permission found for user '%s' on vhost '%s'", username, vhost)
	}
	return nil
}

// HasVHostAccess reports whether a user has access to a vhost.
// Admin users bypass the check and always return true.
func HasVHostAccess(username, vhost string) (bool, error) {
	// Admins have unrestricted access
	u, err := GetUserByUsername(username)
	if err != nil {
		return false, err
	}
	role, err := GetRoleByID(u.RoleID)
	if err != nil {
		return false, err
	}
	if role.Name == "admin" {
		return true, nil
	}

	var count int
	err = db.QueryRow(
		"SELECT COUNT(*) FROM user_vhosts WHERE username = ? AND vhost = ?",
		username, vhost,
	).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	return count > 0, nil
}

// ListUserVHosts returns all vhosts a user has access to.
func ListUserVHosts(username string) ([]string, error) {
	rows, err := db.Query(
		"SELECT vhost FROM user_vhosts WHERE username = ? ORDER BY vhost",
		username,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vhosts []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		vhosts = append(vhosts, v)
	}
	return vhosts, nil
}

// ListVHostUsers returns all users that have access to a vhost.
func ListVHostUsers(vhost string) ([]string, error) {
	rows, err := db.Query(
		"SELECT username FROM user_vhosts WHERE vhost = ? ORDER BY username",
		vhost,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// ListAllPermissions returns every user-vhost grant.
func ListAllPermissions() ([]VHostPermission, error) {
	rows, err := db.Query(
		"SELECT username, vhost FROM user_vhosts ORDER BY vhost, username",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []VHostPermission
	for rows.Next() {
		var p VHostPermission
		if err := rows.Scan(&p.Username, &p.VHost); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	return perms, nil
}
