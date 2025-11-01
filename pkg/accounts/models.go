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

// UpstreamCredential stores user-scoped upstream provider secrets and endpoints.
type UpstreamCredential struct {
	ID        string            `gorm:"type:char(36);primaryKey"`
	UserID    string            `gorm:"type:char(36);index"`
	Provider  string            `gorm:"type:varchar(64);index"`
	Label     string            `gorm:"type:varchar(128)"`
	APIKey    string            `gorm:"type:varchar(255)"`
	Endpoints datatypes.JSON    `gorm:"type:jsonb"`
	Metadata  datatypes.JSONMap `gorm:"type:jsonb"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Validate ensures the upstream credential is well formed.
func (c UpstreamCredential) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return fmt.Errorf("%w: upstream credential user_id empty", ErrInvalidInput)
	}
	if strings.TrimSpace(c.Provider) == "" {
		return fmt.Errorf("%w: upstream credential provider empty", ErrInvalidInput)
	}
	if len(c.Provider) > 64 {
		return fmt.Errorf("%w: provider too long", ErrInvalidInput)
	}
	if len(c.Label) > maxNameLength {
		return fmt.Errorf("%w: upstream credential label too long", ErrInvalidInput)
	}
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("%w: upstream credential api key empty", ErrInvalidInput)
	}
	return nil
}

// UserAPIKeyBinding maps a user API key to a preferred upstream credential.
type UserAPIKeyBinding struct {
	ID                   string            `gorm:"type:char(36);primaryKey"`
	UserID               string            `gorm:"type:char(36);index"`
	UserAPIKeyID         string            `gorm:"type:char(36);uniqueIndex"`
	UpstreamCredentialID string            `gorm:"type:char(36);index"`
	Metadata             datatypes.JSONMap `gorm:"type:jsonb"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Validate ensures the binding is valid.
func (b UserAPIKeyBinding) Validate() error {
	if strings.TrimSpace(b.UserID) == "" {
		return fmt.Errorf("%w: binding user_id empty", ErrInvalidInput)
	}
	if strings.TrimSpace(b.UserAPIKeyID) == "" {
		return fmt.Errorf("%w: binding api_key_id empty", ErrInvalidInput)
	}
	if strings.TrimSpace(b.UpstreamCredentialID) == "" {
		return fmt.Errorf("%w: binding upstream_credential_id empty", ErrInvalidInput)
	}
	return nil
}
