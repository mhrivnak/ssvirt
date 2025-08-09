package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents a user account following VMware Cloud Director API spec
type User struct {
	ID               string         `gorm:"type:varchar(255);primary_key" json:"id"`
	Username         string         `gorm:"unique;not null;size:255" json:"username"`
	FullName         string         `gorm:"not null;size:255" json:"fullName"`
	Description      string         `json:"description"`
	Email            string         `gorm:"unique;not null;size:255" json:"email"`
	PasswordHash     string         `gorm:"not null" json:"-"`
	Password         string         `gorm:"-" json:"password,omitempty"` // Only for input, never stored
	DeployedVmQuota  int            `gorm:"default:0;not null" json:"deployedVmQuota"`
	StoredVmQuota    int            `gorm:"default:0;not null" json:"storedVmQuota"`
	NameInSource     string         `json:"nameInSource"`
	Enabled          bool           `gorm:"default:true;not null" json:"enabled"`
	IsGroupRole      bool           `gorm:"default:false;not null" json:"isGroupRole"`
	ProviderType     string         `gorm:"default:'LOCAL';not null;size:50" json:"providerType"`
	Locked           bool           `gorm:"default:false;not null" json:"locked"`
	Stranded         bool           `gorm:"default:false;not null" json:"stranded"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Entity references (populated in API responses)
	RoleEntityRefs []EntityRef `gorm:"-" json:"roleEntityRefs,omitempty"`
	OrgEntityRef   *EntityRef  `gorm:"-" json:"orgEntityRef,omitempty"`

	// Relationships
	UserRoles []UserRole `gorm:"foreignKey:UserID;references:ID" json:"user_roles,omitempty"`
}

// BeforeCreate sets the URN ID if not already set
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = GenerateUserURN()
	}
	if u.NameInSource == "" {
		u.NameInSource = u.Username
	}
	return nil
}

// SetPassword hashes the provided password and stores it in the PasswordHash field
func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hashedPassword)
	return nil
}

// CheckPassword verifies if the provided password matches the stored hash
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// IsSystemAdmin checks if the user has System Administrator role
func (u *User) IsSystemAdmin() bool {
	for _, roleRef := range u.RoleEntityRefs {
		if roleRef.Name == RoleSystemAdmin {
			return true
		}
	}
	return false
}
