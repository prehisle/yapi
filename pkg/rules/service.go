package rules

import (
	"context"
	"errors"
	"log"
	"sync"
)

// Service 封装业务层逻辑，支持缓存与事件通知。
type Service interface {
	ListRules(ctx context.Context) ([]Rule, error)
	GetRule(ctx context.Context, id string) (Rule, error)
	UpsertRule(ctx context.Context, rule Rule) error
	DeleteRule(ctx context.Context, id string) error
	StartBackgroundSync(ctx context.Context)
}

// ServiceOption 用于配置 service。
type ServiceOption func(*service)

// WithCache 启用缓存。
func WithCache(cache Cache) ServiceOption {
	return func(s *service) {
		s.cache = cache
	}
}

// WithEventBus 设置事件总线，用于多实例同步。
func WithEventBus(bus EventBus) ServiceOption {
	return func(s *service) {
		s.eventBus = bus
	}
}

// WithLogger 设置日志记录器。
func WithLogger(logger *log.Logger) ServiceOption {
	return func(s *service) {
		s.logger = logger
	}
}

// service 实现 Service 接口。
type service struct {
	store    Store
	cache    Cache
	eventBus EventBus

	mu     sync.RWMutex
	cached []Rule
	logger *log.Logger
}

// NewService 返回默认实现。
func NewService(store Store, opts ...ServiceOption) Service {
	s := &service{
		store:  store,
		logger: log.Default(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *service) ListRules(ctx context.Context) ([]Rule, error) {
	if rules, ok := s.getCachedRules(); ok {
		return cloneRules(rules), nil
	}
	if s.cache != nil {
		if rules, err := s.cache.Get(ctx); err == nil {
			s.setCachedRules(rules)
			return cloneRules(rules), nil
		} else if !errors.Is(err, ErrCacheMiss) {
			s.logger.Printf("rules cache get failed: %v", err)
		}
	}
	rules, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	s.setCachedRules(rules)
	if s.cache != nil {
		if err := s.cache.Set(ctx, rules); err != nil {
			s.logger.Printf("rules cache set failed: %v", err)
		}
	}
	return cloneRules(rules), nil
}

func (s *service) GetRule(ctx context.Context, id string) (Rule, error) {
	return s.store.Get(ctx, id)
}

func (s *service) UpsertRule(ctx context.Context, rule Rule) error {
	if err := s.store.Save(ctx, rule); err != nil {
		return err
	}
	if err := s.refreshCache(ctx); err != nil {
		return err
	}
	s.broadcast(ctx)
	return nil
}

func (s *service) DeleteRule(ctx context.Context, id string) error {
	if err := s.store.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.refreshCache(ctx); err != nil {
		return err
	}
	s.broadcast(ctx)
	return nil
}

func (s *service) StartBackgroundSync(ctx context.Context) {
	if s.eventBus == nil {
		return
	}
	events, err := s.eventBus.Subscribe(ctx)
	if err != nil {
		s.logger.Printf("rules event subscribe failed: %v", err)
		return
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				if evt == EventRulesChanged {
					if err := s.reload(ctx); err != nil {
						s.logger.Printf("rules reload failed: %v", err)
					}
				}
			}
		}
	}()
}

func (s *service) broadcast(ctx context.Context) {
	if s.eventBus == nil {
		return
	}
	if err := s.eventBus.Publish(ctx, EventRulesChanged); err != nil {
		s.logger.Printf("rules event publish failed: %v", err)
	}
}

func (s *service) refreshCache(ctx context.Context) error {
	rules, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	s.setCachedRules(rules)
	if s.cache != nil {
		if err := s.cache.Set(ctx, rules); err != nil {
			s.logger.Printf("rules cache set failed: %v", err)
		}
	}
	return nil
}

func (s *service) reload(ctx context.Context) error {
	if s.cache != nil {
		if rules, err := s.cache.Get(ctx); err == nil {
			s.setCachedRules(rules)
			return nil
		}
	}
	rules, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	s.setCachedRules(rules)
	if s.cache != nil {
		if err := s.cache.Set(ctx, rules); err != nil {
			s.logger.Printf("rules cache set failed: %v", err)
		}
	}
	return nil
}

func (s *service) getCachedRules() ([]Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.cached) == 0 {
		return nil, false
	}
	return s.cached, true
}

func (s *service) setCachedRules(rules []Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cached = cloneRules(rules)
}

func cloneRules(src []Rule) []Rule {
	dst := make([]Rule, len(src))
	for i := range src {
		dst[i] = cloneRule(src[i])
	}
	return dst
}

func cloneRule(r Rule) Rule {
	cloned := r
	cloned.Matcher.Methods = append([]string(nil), r.Matcher.Methods...)
	if len(r.Matcher.Headers) > 0 {
		cloned.Matcher.Headers = make(map[string]string, len(r.Matcher.Headers))
		for k, v := range r.Matcher.Headers {
			cloned.Matcher.Headers[k] = v
		}
	}
	if len(r.Actions.SetHeaders) > 0 {
		cloned.Actions.SetHeaders = make(map[string]string, len(r.Actions.SetHeaders))
		for k, v := range r.Actions.SetHeaders {
			cloned.Actions.SetHeaders[k] = v
		}
	}
	if len(r.Actions.AddHeaders) > 0 {
		cloned.Actions.AddHeaders = make(map[string]string, len(r.Actions.AddHeaders))
		for k, v := range r.Actions.AddHeaders {
			cloned.Actions.AddHeaders[k] = v
		}
	}
	cloned.Actions.RemoveHeaders = append([]string(nil), r.Actions.RemoveHeaders...)
	if r.Actions.RewritePathRegex != nil {
		rewrite := *r.Actions.RewritePathRegex
		cloned.Actions.RewritePathRegex = &rewrite
	}
	return cloned
}
