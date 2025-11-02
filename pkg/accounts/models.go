package accounts

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	maxNameLength        = 128
	maxDescriptionLength = 512
	apiKeyPrefixLength   = 8
)

// User represents an API consumer.
type User struct {
	ID          string            `gorm:"type:char(36);primaryKey"`
	Name        string            `gorm:"type:varchar(128);uniqueIndex"`
	Description string            `gorm:"type:varchar(512)"`
	Metadata    datatypes.JSONMap `gorm:"type:jsonb"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

// Validate checks the user payload.
func (u User) Validate() error {
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("%w: user name must not be empty", ErrInvalidInput)
	}
	if len(u.Name) > maxNameLength {
		return fmt.Errorf("%w: user name too long", ErrInvalidInput)
	}
	if len(u.Description) > maxDescriptionLength {
		return fmt.Errorf("%w: user description too long", ErrInvalidInput)
	}
	return nil
}

// APIKey represents a generated access token bound to a user.
type APIKey struct {
	ID         string `gorm:"type:char(36);primaryKey"`
	UserID     string `gorm:"type:char(36);index"`
	Label      string `gorm:"type:varchar(128)"`
	Prefix     string `gorm:"type:char(8);uniqueIndex"`
	SecretHash string `gorm:"type:varchar(255)"`
	Enabled    bool   `gorm:"type:boolean;default:true"`
	LastUsedAt *time.Time
	Metadata   datatypes.JSONMap `gorm:"type:jsonb"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

// Validate ensures APIKey has the required attributes.
func (k APIKey) Validate() error {
	if strings.TrimSpace(k.UserID) == "" {
		return fmt.Errorf("%w: api key user_id empty", ErrInvalidInput)
	}
	if len(k.Label) > maxNameLength {
		return fmt.Errorf("%w: api key label too long", ErrInvalidInput)
	}
	if len(k.Prefix) != apiKeyPrefixLength {
		return fmt.Errorf("%w: api key prefix malformed", ErrInvalidInput)
	}
	if strings.TrimSpace(k.SecretHash) == "" {
		return fmt.Errorf("%w: api key secret hash empty", ErrInvalidInput)
	}
	return nil
}

// UpstreamKey stores upstream secrets, endpoints, and service metadata.
type UpstreamKey struct {
	ID        string            `gorm:"type:char(36);primaryKey"`
	UserID    string            `gorm:"type:char(36);index"`
	Service   string            `gorm:"type:varchar(64);index;column:provider"`
	Name      string            `gorm:"type:varchar(128);column:label"`
	APIKey    string            `gorm:"type:varchar(255)"`
	Endpoints datatypes.JSON    `gorm:"type:jsonb"`
	Metadata  datatypes.JSONMap `gorm:"type:jsonb"`
	Enabled   bool              `gorm:"type:boolean;default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName retains the legacy table name.
func (UpstreamKey) TableName() string {
	return "upstream_credentials"
}

// Validate ensures the upstream key is well formed.
func (k UpstreamKey) Validate() error {
	if strings.TrimSpace(k.UserID) == "" {
		return fmt.Errorf("%w: upstream credential user_id empty", ErrInvalidInput)
	}
	if strings.TrimSpace(k.Service) == "" {
		return fmt.Errorf("%w: upstream credential service empty", ErrInvalidInput)
	}
	if len(k.Service) > 64 {
		return fmt.Errorf("%w: service too long", ErrInvalidInput)
	}
	if len(k.Name) > maxNameLength {
		return fmt.Errorf("%w: upstream credential name too long", ErrInvalidInput)
	}
	if strings.TrimSpace(k.APIKey) == "" {
		return fmt.Errorf("%w: upstream credential api key empty", ErrInvalidInput)
	}
	return nil
}

// UserKeyBinding maps a user API key to upstream keys per service.
type UserKeyBinding struct {
	ID            string            `gorm:"type:char(36);primaryKey"`
	UserID        string            `gorm:"type:char(36);index"`
	UserAPIKeyID  string            `gorm:"type:char(36);index;uniqueIndex:user_key_service"`
	UpstreamKeyID string            `gorm:"type:char(36);index;column:upstream_credential_id"`
	Service       string            `gorm:"type:varchar(64);index;uniqueIndex:user_key_service"`
	Position      int               `gorm:"type:int;default:0"`
	Metadata      datatypes.JSONMap `gorm:"type:jsonb"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TableName retains the legacy table name.
func (UserKeyBinding) TableName() string {
	return "user_api_key_bindings"
}

// Validate ensures the binding is valid.
func (b UserKeyBinding) Validate() error {
	if strings.TrimSpace(b.UserID) == "" {
		return fmt.Errorf("%w: binding user_id empty", ErrInvalidInput)
	}
	if strings.TrimSpace(b.UserAPIKeyID) == "" {
		return fmt.Errorf("%w: binding api_key_id empty", ErrInvalidInput)
	}
	if strings.TrimSpace(b.UpstreamKeyID) == "" {
		return fmt.Errorf("%w: binding upstream_key_id empty", ErrInvalidInput)
	}
	if strings.TrimSpace(b.Service) == "" {
		return fmt.Errorf("%w: binding service empty", ErrInvalidInput)
	}
	return nil
}

// Temporary aliases to ease migration from legacy naming.
type UpstreamCredential = UpstreamKey
type UserAPIKeyBinding = UserKeyBinding
