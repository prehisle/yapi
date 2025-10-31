package rules

import "context"

// Service 封装业务层逻辑，后续可扩展缓存、事件通知等能力。
type Service interface {
	ListRules(ctx context.Context) ([]Rule, error)
	GetRule(ctx context.Context, id string) (Rule, error)
	UpsertRule(ctx context.Context, rule Rule) error
	DeleteRule(ctx context.Context, id string) error
}

// service 实现 Service 接口。
type service struct {
	store Store
}

// NewService 返回默认实现。
func NewService(store Store) Service {
	return &service{store: store}
}

func (s *service) ListRules(ctx context.Context) ([]Rule, error) {
	return s.store.List(ctx)
}

func (s *service) GetRule(ctx context.Context, id string) (Rule, error) {
	return s.store.Get(ctx, id)
}

func (s *service) UpsertRule(ctx context.Context, rule Rule) error {
	return s.store.Save(ctx, rule)
}

func (s *service) DeleteRule(ctx context.Context, id string) error {
	return s.store.Delete(ctx, id)
}
