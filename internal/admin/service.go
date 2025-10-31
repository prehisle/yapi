package admin

import (
	"context"

	"github.com/prehisle/yapi/pkg/rules"
)

// Service 定义管理端对规则的操作接口。
type Service interface {
	ListRules(ctx context.Context) ([]rules.Rule, error)
	GetRule(ctx context.Context, id string) (rules.Rule, error)
	CreateOrUpdateRule(ctx context.Context, rule rules.Rule) error
	DeleteRule(ctx context.Context, id string) error
}

type service struct {
	rules rules.Service
}

// NewService 创建管理端默认实现。
func NewService(rules rules.Service) Service {
	return &service{rules: rules}
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
