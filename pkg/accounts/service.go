package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	apiKeyPrefix               = "yapi"
	apiKeySecretBytes          = 24
	userAPIKeySecretHashCost   = bcrypt.DefaultCost
	defaultAPIKeyPrefixSegment = 4
)

// Service exposes account management operations.
type Service interface {
	AutoMigrate(ctx context.Context) error

	CreateUser(ctx context.Context, params CreateUserParams) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	GetUser(ctx context.Context, id string) (User, error)
	DeleteUser(ctx context.Context, id string) error

	CreateUserAPIKey(ctx context.Context, params CreateAPIKeyParams) (APIKey, string, error)
	ListUserAPIKeys(ctx context.Context, userID string) ([]APIKey, error)
	SetUserAPIKeyEnabled(ctx context.Context, apiKeyID string, enabled bool) error
	RevokeUserAPIKey(ctx context.Context, apiKeyID string) error

	CreateUpstreamKey(ctx context.Context, params CreateUpstreamKeyParams) (UpstreamKey, error)
	ListUpstreamKeys(ctx context.Context, userID string) ([]UpstreamKey, error)
	SetUpstreamKeyEnabled(ctx context.Context, upstreamKeyID string, enabled bool) error
	DeleteUpstreamKey(ctx context.Context, upstreamKeyID string) error

	UpsertBinding(ctx context.Context, params UpsertBindingParams) (UserKeyBinding, error)
	ListBindingsByAPIKey(ctx context.Context, apiKeyID string) ([]BindingWithUpstream, error)
	DeleteBinding(ctx context.Context, bindingID string) error

	ResolveAPIKey(ctx context.Context, rawKey string) (APIKey, error)
}

// CreateUserParams defines the payload for user creation.
type CreateUserParams struct {
	Name        string
	Description string
	Metadata    map[string]any
}

// CreateAPIKeyParams defines the payload for API key generation.
type CreateAPIKeyParams struct {
	UserID string
	Label  string
}

// CreateUpstreamCredentialParams describes an upstream credential creation.
type CreateUpstreamKeyParams struct {
	UserID    string
	Service   string
	Name      string
	Plaintext string
	Endpoints []string
	Metadata  map[string]any
}

// UpsertBindingParams maps a user API key to an upstream key for a service.
type UpsertBindingParams struct {
	UserID         string
	UserAPIKeyID   string
	UpstreamKeyID  string
	Service        string
	Position       int
	Metadata       map[string]any
}

// BindingWithUpstream represents a binding alongside its upstream details.
type BindingWithUpstream struct {
	Binding  UserKeyBinding
	Upstream UpstreamKey
}

type service struct {
	db *gorm.DB
}

// NewService constructs a Service backed by the provided gorm DB.
func NewService(db *gorm.DB) Service {
	return &service{db: db}
}

func (s *service) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(
		&User{},
		&APIKey{},
		&UpstreamKey{},
		&UserKeyBinding{},
	)
}

func (s *service) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	user := User{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(params.Name),
		Description: strings.TrimSpace(params.Description),
	}
	if params.Metadata != nil {
		user.Metadata = datatypes.JSONMap(params.Metadata)
	}
	if err := user.Validate(); err != nil {
		return User{}, err
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate") {
			return User{}, fmt.Errorf("%w: user name already exists", ErrConflict)
		}
		return User{}, err
	}
	return user, nil
}

func (s *service) ListUsers(ctx context.Context) ([]User, error) {
	var users []User
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (s *service) GetUser(ctx context.Context, id string) (User, error) {
	var user User
	err := s.db.WithContext(ctx).First(&user, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *service) DeleteUser(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&User{}).Error; err != nil {
		return err
	}
	return nil
}

func (s *service) CreateUserAPIKey(ctx context.Context, params CreateAPIKeyParams) (APIKey, string, error) {
	if strings.TrimSpace(params.UserID) == "" {
		return APIKey{}, "", fmt.Errorf("%w: user_id required", ErrInvalidInput)
	}
	if _, err := s.GetUser(ctx, params.UserID); err != nil {
		return APIKey{}, "", err
	}
	plain, prefix, err := generateAPIKey()
	if err != nil {
		return APIKey{}, "", err
	}
	hash, err := hashSecret(plain, userAPIKeySecretHashCost)
	if err != nil {
		return APIKey{}, "", err
	}

	key := APIKey{
		ID:         uuid.NewString(),
		UserID:     params.UserID,
		Label:      strings.TrimSpace(params.Label),
		Prefix:     prefix,
		SecretHash: hash,
		Enabled:    true,
	}
	if err := key.Validate(); err != nil {
		return APIKey{}, "", err
	}
	if err := s.db.WithContext(ctx).Create(&key).Error; err != nil {
		return APIKey{}, "", err
	}
	return key, plain, nil
}

func (s *service) ListUserAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	var keys []APIKey
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *service) SetUserAPIKeyEnabled(ctx context.Context, apiKeyID string, enabled bool) error {
	if strings.TrimSpace(apiKeyID) == "" {
		return fmt.Errorf("%w: api_key_id required", ErrInvalidInput)
	}
	result := s.db.WithContext(ctx).Model(&APIKey{}).Where("id = ?", apiKeyID).Updates(map[string]any{
		"enabled":    enabled,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *service) RevokeUserAPIKey(ctx context.Context, apiKeyID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("user_api_key_id = ?", apiKeyID).Delete(&UserKeyBinding{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", apiKeyID).Delete(&APIKey{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *service) CreateUpstreamKey(ctx context.Context, params CreateUpstreamKeyParams) (UpstreamKey, error) {
	if strings.TrimSpace(params.UserID) == "" {
		return UpstreamKey{}, fmt.Errorf("%w: user_id required", ErrInvalidInput)
	}
	if _, err := s.GetUser(ctx, params.UserID); err != nil {
		return UpstreamKey{}, err
	}
	key := UpstreamKey{
		ID:      uuid.NewString(),
		UserID:  params.UserID,
		Service: strings.TrimSpace(params.Service),
		Name:    strings.TrimSpace(params.Name),
		APIKey:  strings.TrimSpace(params.Plaintext),
		Enabled: true,
	}
	if len(params.Endpoints) > 0 {
		endpointsJSON, encodeErr := json.Marshal(params.Endpoints)
		if encodeErr != nil {
			return UpstreamKey{}, encodeErr
		}
		key.Endpoints = datatypes.JSON(endpointsJSON)
	}
	if params.Metadata != nil {
		key.Metadata = datatypes.JSONMap(params.Metadata)
	}
	if err := key.Validate(); err != nil {
		return UpstreamKey{}, err
	}
	if err := s.db.WithContext(ctx).Create(&key).Error; err != nil {
		return UpstreamKey{}, err
	}
	return key, nil
}

func (s *service) ListUpstreamKeys(ctx context.Context, userID string) ([]UpstreamKey, error) {
	var keys []UpstreamKey
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *service) SetUpstreamKeyEnabled(ctx context.Context, upstreamKeyID string, enabled bool) error {
	if strings.TrimSpace(upstreamKeyID) == "" {
		return fmt.Errorf("%w: upstream_key_id required", ErrInvalidInput)
	}
	result := s.db.WithContext(ctx).Model(&UpstreamKey{}).Where("id = ?", upstreamKeyID).Updates(map[string]any{
		"enabled":    enabled,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *service) DeleteUpstreamKey(ctx context.Context, upstreamKeyID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("upstream_credential_id = ?", upstreamKeyID).Delete(&UserKeyBinding{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", upstreamKeyID).Delete(&UpstreamKey{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *service) UpsertBinding(ctx context.Context, params UpsertBindingParams) (UserKeyBinding, error) {
	if strings.TrimSpace(params.UserID) == "" {
		return UserKeyBinding{}, fmt.Errorf("%w: user_id required", ErrInvalidInput)
	}
	if strings.TrimSpace(params.Service) == "" {
		return UserKeyBinding{}, fmt.Errorf("%w: service required", ErrInvalidInput)
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var apiKey APIKey
		if err := tx.WithContext(ctx).First(&apiKey, "id = ?", params.UserAPIKeyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: api key not found", ErrNotFound)
			}
			return err
		}
		var upstream UpstreamKey
		if err := tx.WithContext(ctx).First(&upstream, "id = ?", params.UpstreamKeyID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("%w: upstream key not found", ErrNotFound)
			}
			return err
		}
		if apiKey.UserID != params.UserID || upstream.UserID != params.UserID {
			return fmt.Errorf("%w: binding ownership mismatch", ErrConflict)
		}

		targetService := strings.TrimSpace(params.Service)
		var binding UserKeyBinding
		err := tx.WithContext(ctx).Where("user_api_key_id = ? AND service = ?", params.UserAPIKeyID, targetService).
			First(&binding).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			binding = UserKeyBinding{
				ID:            uuid.NewString(),
				UserID:        params.UserID,
				UserAPIKeyID:  params.UserAPIKeyID,
				UpstreamKeyID: params.UpstreamKeyID,
				Service:       targetService,
				Position:      params.Position,
			}
			if params.Metadata != nil {
				binding.Metadata = datatypes.JSONMap(params.Metadata)
			}
			if err := binding.Validate(); err != nil {
				return err
			}
			return tx.WithContext(ctx).Create(&binding).Error
		}
		if err != nil {
			return err
		}

		binding.UpstreamKeyID = params.UpstreamKeyID
		binding.Position = params.Position
		if params.Metadata != nil {
			binding.Metadata = datatypes.JSONMap(params.Metadata)
		}
		if err := binding.Validate(); err != nil {
			return err
		}
		return tx.WithContext(ctx).Model(&UserKeyBinding{}).Where("id = ?", binding.ID).Updates(map[string]any{
			"upstream_credential_id": binding.UpstreamKeyID,
			"position":               binding.Position,
			"metadata":               binding.Metadata,
			"updated_at":             time.Now(),
		}).Error
	})
	if err != nil {
		return UserKeyBinding{}, err
}

func (s *service) ResolveAPIKey(ctx context.Context, rawKey string) (APIKey, error) {
	prefix, secret, err := splitAPIKey(rawKey)
	if err != nil {
		return APIKey{}, err
	}
	var key APIKey
	err = s.db.WithContext(ctx).First(&key, "prefix = ?", prefix).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return APIKey{}, ErrNotFound
	}
	if err != nil {
		return APIKey{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(key.SecretHash), []byte(secret)); err != nil {
		return APIKey{}, ErrNotFound
	}
	return key, nil
}

func (s *service) ResolveBindingByRawKey(ctx context.Context, rawKey string) (UserAPIKeyBinding, UpstreamCredential, error) {
	key, err := s.ResolveAPIKey(ctx, rawKey)
	if err != nil {
		return UserAPIKeyBinding{}, UpstreamCredential{}, err
	}
	return s.GetBindingByAPIKeyID(ctx, key.ID)
}

func generateAPIKey() (string, string, error) {
	buff := make([]byte, apiKeySecretBytes)
	if _, err := rand.Read(buff); err != nil {
		return "", "", err
	}
	prefix := hex.EncodeToString(buff[:defaultAPIKeyPrefixSegment])
	secretSegment := hex.EncodeToString(buff[defaultAPIKeyPrefixSegment:])
	plain := fmt.Sprintf("%s_%s_%s", apiKeyPrefix, prefix, secretSegment)
	return plain, prefix, nil
}

func splitAPIKey(raw string) (string, string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", "", fmt.Errorf("%w: api key empty", ErrInvalidInput)
	}
	parts := strings.Split(raw, "_")
	if len(parts) != 3 || parts[0] != apiKeyPrefix || len(parts[1]) != apiKeyPrefixLength {
		return "", "", fmt.Errorf("%w: api key format invalid", ErrInvalidInput)
	}
	return parts[1], raw, nil
}

func hashSecret(secret string, cost int) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("%w: secret empty", ErrInvalidInput)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(secret), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
