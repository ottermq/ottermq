package persistdb

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   int    `json:"role_id"`
}

type Role struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Permission struct {
	ID       int    `json:"id"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

type RolePermission struct {
	RoleID       int `json:"role_id"`
	PermissionID int `json:"permission_id"`
}

type UserListDTO struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	HasPassword bool   `json:"has_password"`
	Role        string `json:"role"`
}

type UserCreateDTO struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	RoleID          int    `json:"role"`
}

type VHostPermission struct {
	Username string `json:"username"`
	VHost    string `json:"vhost"`
}
