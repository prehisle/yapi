package admin

import (
	"context"
	"errors"

	"github.com/prehisle/yapi/pkg/accounts"
	"github.com/prehisle/yapi/pkg/rules"
)

var ErrAccountsUnavailable = errors.New("accounts service unavailable")

// Service 定义管理端对规则的操作接口。
type Service interface {
	ListRules(ctx context.Context) ([]rules.Rule, error)
	GetRule(ctx context.Context, id string) (rules.Rule, error)
	CreateOrUpdateRule(ctx context.Context, rule rules.Rule) error
	DeleteRule(ctx context.Context, id string) error

	CreateUser(ctx context.Context, params accounts.CreateUserParams) (accounts.User, error)
	ListUsers(ctx context.Context) ([]accounts.User, error)
	DeleteUser(ctx context.Context, id string) error

	CreateUserAPIKey(ctx context.Context, params accounts.CreateAPIKeyParams) (accounts.APIKey, string, error)
	ListUserAPIKeys(ctx context.Context, userID string) ([]accounts.APIKey, error)
	RevokeUserAPIKey(ctx context.Context, apiKeyID string) error

	CreateUpstreamCredential(ctx context.Context, params accounts.CreateUpstreamCredentialParams) (accounts.UpstreamCredential, error)
	UpdateUpstreamCredential(ctx context.Context, params accounts.UpdateUpstreamCredentialParams) (accounts.UpstreamCredential, error)
	ListUpstreamCredentials(ctx context.Context, userID string) ([]accounts.UpstreamCredential, error)
	DeleteUpstreamCredential(ctx context.Context, credentialID string) error

	BindAPIKey(ctx context.Context, params accounts.BindAPIKeyParams) (accounts.UserAPIKeyBinding, error)
	GetBindingByAPIKeyID(ctx context.Context, apiKeyID string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error)
}

type service struct {
	rules    rules.Service
	accounts accounts.Service
}

// NewService 创建管理端默认实现。
func NewService(rules rules.Service, accounts accounts.Service) Service {
	return &service{rules: rules, accounts: accounts}
}

func (s *service) ListRules(ctx context.Context) ([]rules.Rule, error) {
	return s.rules.ListRules(ctx)
}

func (s *service) GetRule(ctx context.Context, id string) (rules.Rule, error) {
	return s.rules.GetRule(ctx, id)
}

func (s *service) CreateOrUpdateRule(ctx context.Context, rule rules.Rule) error {
	return s.rules.UpsertRule(ctx, rule)
}

func (s *service) DeleteRule(ctx context.Context, id string) error {
	return s.rules.DeleteRule(ctx, id)
}

func (s *service) CreateUser(ctx context.Context, params accounts.CreateUserParams) (accounts.User, error) {
	if s.accounts == nil {
		return accounts.User{}, ErrAccountsUnavailable
	}
	return s.accounts.CreateUser(ctx, params)
}

func (s *service) ListUsers(ctx context.Context) ([]accounts.User, error) {
	if s.accounts == nil {
		return nil, ErrAccountsUnavailable
	}
	return s.accounts.ListUsers(ctx)
}

func (s *service) DeleteUser(ctx context.Context, id string) error {
	if s.accounts == nil {
		return ErrAccountsUnavailable
	}
	return s.accounts.DeleteUser(ctx, id)
}

func (s *service) CreateUserAPIKey(ctx context.Context, params accounts.CreateAPIKeyParams) (accounts.APIKey, string, error) {
	if s.accounts == nil {
		return accounts.APIKey{}, "", ErrAccountsUnavailable
	}
	return s.accounts.CreateUserAPIKey(ctx, params)
}

func (s *service) ListUserAPIKeys(ctx context.Context, userID string) ([]accounts.APIKey, error) {
	if s.accounts == nil {
		return nil, ErrAccountsUnavailable
	}
	return s.accounts.ListUserAPIKeys(ctx, userID)
}

func (s *service) RevokeUserAPIKey(ctx context.Context, apiKeyID string) error {
	if s.accounts == nil {
		return ErrAccountsUnavailable
	}
	return s.accounts.RevokeUserAPIKey(ctx, apiKeyID)
}

func (s *service) CreateUpstreamCredential(ctx context.Context, params accounts.CreateUpstreamCredentialParams) (accounts.UpstreamCredential, error) {
	if s.accounts == nil {
		return accounts.UpstreamCredential{}, ErrAccountsUnavailable
	}
	return s.accounts.CreateUpstreamCredential(ctx, params)
}

func (s *service) UpdateUpstreamCredential(ctx context.Context, params accounts.UpdateUpstreamCredentialParams) (accounts.UpstreamCredential, error) {
	if s.accounts == nil {
		return accounts.UpstreamCredential{}, ErrAccountsUnavailable
	}
	return s.accounts.UpdateUpstreamCredential(ctx, params)
}

func (s *service) ListUpstreamCredentials(ctx context.Context, userID string) ([]accounts.UpstreamCredential, error) {
	if s.accounts == nil {
		return nil, ErrAccountsUnavailable
	}
	return s.accounts.ListUpstreamCredentials(ctx, userID)
}

func (s *service) DeleteUpstreamCredential(ctx context.Context, credentialID string) error {
	if s.accounts == nil {
		return ErrAccountsUnavailable
	}
	return s.accounts.DeleteUpstreamCredential(ctx, credentialID)
}

func (s *service) BindAPIKey(ctx context.Context, params accounts.BindAPIKeyParams) (accounts.UserAPIKeyBinding, error) {
	if s.accounts == nil {
		return accounts.UserAPIKeyBinding{}, ErrAccountsUnavailable
	}
	return s.accounts.BindAPIKey(ctx, params)
}

func (s *service) GetBindingByAPIKeyID(ctx context.Context, apiKeyID string) (accounts.UserAPIKeyBinding, accounts.UpstreamCredential, error) {
	if s.accounts == nil {
		return accounts.UserAPIKeyBinding{}, accounts.UpstreamCredential{}, ErrAccountsUnavailable
	}
	return s.accounts.GetBindingByAPIKeyID(ctx, apiKeyID)
}
