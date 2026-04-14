package persistdb

import (
	"github.com/rs/zerolog/log"
)

var defaultRoles = []Role{
	{Name: "admin", Description: "Can configure settings, create/delete resources, manage users, etc."},
	{Name: "user", Description: "Can read and write to resources but cannot manage users or settings."},
	{Name: "guest", Description: "Can only read resources"},
}

func AddDefaultRoles() {
	// Add roles to the database
	for _, role := range defaultRoles {
		_, err := db.Exec("INSERT OR IGNORE INTO roles (name, description) VALUES (?, ?)", role.Name, role.Description)
		if err != nil {
			log.Error().Err(err).Msg("Failed to insert role")
		}
	}
}

func GetRoleByID(id int) (Role, error) {
	var role Role
	err := db.QueryRow("SELECT id, name, description FROM roles WHERE id = ?", id).Scan(&role.ID, &role.Name, &role.Description)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query role")
		return Role{}, err
	}
	return role, nil
}
