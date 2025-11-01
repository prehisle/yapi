package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	RevokeUserAPIKey(ctx context.Context, apiKeyID string) error

	CreateUpstreamCredential(ctx context.Context, params CreateUpstreamCredentialParams) (UpstreamCredential, error)
	ListUpstreamCredentials(ctx context.Context, userID string) ([]UpstreamCredential, error)
	DeleteUpstreamCredential(ctx context.Context, credentialID string) error

	BindAPIKey(ctx context.Context, params BindAPIKeyParams) (UserAPIKeyBinding, error)
	GetBindingByAPIKeyID(ctx context.Context, apiKeyID string) (UserAPIKeyBinding, UpstreamCredential, error)
	ResolveAPIKey(ctx context.Context, rawKey string) (APIKey, error)
	ResolveBindingByRawKey(ctx context.Context, rawKey string) (UserAPIKeyBinding, UpstreamCredential, error)
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
type CreateUpstreamCredentialParams struct {
	UserID    string
	Provider  string
	Label     string
	Plaintext string
	Endpoints []string
	Metadata  map[string]any
}

// BindAPIKeyParams maps a user API key to an upstream credential.
type BindAPIKeyParams struct {
	UserID               string
	UserAPIKeyID         string
	UpstreamCredentialID string
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
		&UpstreamCredential{},
		&UserAPIKeyBinding{},
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

func (s *service) RevokeUserAPIKey(ctx context.Context, apiKeyID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("user_api_key_id = ?", apiKeyID).Delete(&UserAPIKeyBinding{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", apiKeyID).Delete(&APIKey{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *service) CreateUpstreamCredential(ctx context.Context, params CreateUpstreamCredentialParams) (UpstreamCredential, error) {
    if strings.TrimSpace(params.UserID) == "" {
        return UpstreamCredential{}, fmt.Errorf("%w: user_id required", ErrInvalidInput)
    }
    if _, err := s.GetUser(ctx, params.UserID); err != nil {
        return UpstreamCredential{}, err
    }
    cred := UpstreamCredential{
        ID:         uuid.NewString(),
        UserID:     params.UserID,
        Provider:   strings.TrimSpace(params.Provider),
        Label:      strings.TrimSpace(params.Label),
        APIKey:     strings.TrimSpace(params.Plaintext),
	}
	if len(params.Endpoints) > 0 {
		endpointsJSON, encodeErr := json.Marshal(params.Endpoints)
		if encodeErr != nil {
			return UpstreamCredential{}, encodeErr
		}
		cred.Endpoints = datatypes.JSON(endpointsJSON)
	}
	if params.Metadata != nil {
		cred.Metadata = datatypes.JSONMap(params.Metadata)
	}
	if err := cred.Validate(); err != nil {
		return UpstreamCredential{}, err
	}
	if err := s.db.WithContext(ctx).Create(&cred).Error; err != nil {
		return UpstreamCredential{}, err
	}
	return cred, nil
}

func (s *service) ListUpstreamCredentials(ctx context.Context, userID string) ([]UpstreamCredential, error) {
	var creds []UpstreamCredential
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at ASC").Find(&creds).Error; err != nil {
		return nil, err
	}
	return creds, nil
}

func (s *service) DeleteUpstreamCredential(ctx context.Context, credentialID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Where("upstream_credential_id = ?", credentialID).Delete(&UserAPIKeyBinding{}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("id = ?", credentialID).Delete(&UpstreamCredential{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *service) BindAPIKey(ctx context.Context, params BindAPIKeyParams) (UserAPIKeyBinding, error) {
	if strings.TrimSpace(params.UserID) == "" {
		return UserAPIKeyBinding{}, fmt.Errorf("%w: user_id required", ErrInvalidInput)
	}
	var key APIKey
	if err := s.db.WithContext(ctx).First(&key, "id = ?", params.UserAPIKeyID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UserAPIKeyBinding{}, fmt.Errorf("%w: api key not found", ErrNotFound)
		}
		return UserAPIKeyBinding{}, err
	}
	var cred UpstreamCredential
	if err := s.db.WithContext(ctx).First(&cred, "id = ?", params.UpstreamCredentialID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return UserAPIKeyBinding{}, fmt.Errorf("%w: upstream credential not found", ErrNotFound)
		}
		return UserAPIKeyBinding{}, err
	}
	if key.UserID != params.UserID || cred.UserID != params.UserID {
		return UserAPIKeyBinding{}, fmt.Errorf("%w: binding ownership mismatch", ErrConflict)
	}
	binding := UserAPIKeyBinding{
		ID:                   uuid.NewString(),
		UserID:               params.UserID,
		UserAPIKeyID:         params.UserAPIKeyID,
		UpstreamCredentialID: params.UpstreamCredentialID,
	}
	if err := binding.Validate(); err != nil {
		return UserAPIKeyBinding{}, err
	}
	if err := s.db.WithContext(ctx).Save(&binding).Error; err != nil {
		return UserAPIKeyBinding{}, err
	}
	return binding, nil
}

func (s *service) GetBindingByAPIKeyID(ctx context.Context, apiKeyID string) (UserAPIKeyBinding, UpstreamCredential, error) {
	var binding UserAPIKeyBinding
	err := s.db.WithContext(ctx).First(&binding, "user_api_key_id = ?", apiKeyID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UserAPIKeyBinding{}, UpstreamCredential{}, ErrNotFound
	}
	if err != nil {
		return UserAPIKeyBinding{}, UpstreamCredential{}, err
	}
	var cred UpstreamCredential
	if err := s.db.WithContext(ctx).First(&cred, "id = ?", binding.UpstreamCredentialID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return binding, UpstreamCredential{}, ErrNotFound
		}
		return binding, UpstreamCredential{}, err
	}
	return binding, cred, nil
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
