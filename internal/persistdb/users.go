package persistdb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func AddUser(user UserCreateDTO) error {
	hashedPassword, err := hashPassword(user.Password)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		return err
	}
	_, err = db.Exec("INSERT INTO users (username, password, role_id) VALUES (?, ?, ?)", user.Username, hashedPassword, user.RoleID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to insert user")
		return err
	}
	return nil
}

func GetUsers() ([]User, error) {
	rows, err := db.Query("SELECT id, username, role_id FROM users")
	if err != nil {
		log.Error().Err(err).Msg("Failed to query users")
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	for rows.Next() {
		var user User
		err := rows.Scan(&user.ID, &user.Username, &user.RoleID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan user")
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func GetUserByUsername(username string) (User, error) {
	var user User
	err := db.QueryRow("SELECT id, username, role_id FROM users WHERE username = ?", username).Scan(&user.ID, &user.Username, &user.RoleID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query user")
		return User{}, err
	}
	return user, nil
}

func GenerateJWTToken(user UserListDTO, jwtSecret string) (string, error) {
	// convert user to json
	jsonUser, err := json.Marshal(user)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal user")
	}
	// convert jsonUser to base64
	encoded := base64.StdEncoding.EncodeToString(jsonUser)

	claims := jwt.MapClaims{
		"iss":         "ottermq",
		"sub":         user.ID,
		"aud":         "ottermq",
		"exp":         jwt.NewNumericDate(time.Now().Add(time.Hour * 24)),
		"nbf":         jwt.NewNumericDate(time.Now()),
		"iat":         jwt.NewNumericDate(time.Now()),
		"jti":         uuid.New().String(),
		"username":    user.Username,
		"role":        user.Role,
		"encodeduser": encoded,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

var bcryptCost = 14

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(bytes), err
}

func DeleteUser(username string) error {
	result, err := db.Exec("DELETE FROM users WHERE username = ?", username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to delete user")
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}
	return nil
}

func ChangePassword(username, newPassword string) error {
	hashed, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	result, err := db.Exec("UPDATE users SET password = ? WHERE username = ?", hashed, username)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update password")
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user '%s' not found", username)
	}
	return nil
}

// GetUsersWithHashes returns all users including their bcrypt password hashes.
// Used for definitions export.
func GetUsersWithHashes() ([]User, error) {
	rows, err := db.Query("SELECT id, username, password, role_id FROM users")
	if err != nil {
		log.Error().Err(err).Msg("Failed to query users with hashes")
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.RoleID); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

// AddUserWithHash inserts a user using an already-hashed password (e.g. from a definitions import).
// Silently skips the insert when the username already exists.
func AddUserWithHash(username, hash string, roleID int) error {
	_, err := db.Exec(
		"INSERT OR IGNORE INTO users (username, password, role_id) VALUES (?, ?, ?)",
		username, hash, roleID,
	)
	if err != nil {
		log.Error().Err(err).Str("username", username).Msg("Failed to insert user with hash")
	}
	return err
}

func (u User) ToUserListDTO() (UserListDTO, error) {
	role, err := GetRoleByID(u.RoleID)
	if err != nil {
		return UserListDTO{}, err
	}
	return UserListDTO{
		ID:          u.ID,
		Username:    u.Username,
		HasPassword: true,
		Role:        role.Name,
	}, nil
}
