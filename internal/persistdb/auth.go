package persistdb

import (
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

func AuthenticateUser(username, password string) (bool, error) {
	var storedPassword string
	err := db.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return comparePasswords(password, storedPassword)
}

func comparePasswords(password, storedPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password))
	return err == nil, nil
}
