package models

import "github.com/ottermq/ottermq/internal/persistdb"

// From the DTO your DB layer already returns
func FromPersistUserListDTO(dto persistdb.UserListDTO) UserSummary {
	return UserSummary{
		ID:          dto.ID,
		Username:    dto.Username,
		HasPassword: dto.HasPassword,
		Role:        dto.Role,
	}
}

// If you ever need to expose a single user based on persistdb.User + role name
func FromPersistUser(u persistdb.User, roleName string, hasPassword bool) UserSummary {
	return UserSummary{
		ID:          u.ID,
		Username:    u.Username,
		HasPassword: hasPassword,
		Role:        roleName,
	}
}

// Convert API request -> persist layer DTO
func (r UserCreateRequest) ToPersist() persistdb.UserCreateDTO {
	return persistdb.UserCreateDTO{
		Username:        r.Username,
		Password:        r.Password,
		ConfirmPassword: r.ConfirmPassword,
		RoleID:          r.Role, // persistdb expects "role" as RoleID
	}
}
