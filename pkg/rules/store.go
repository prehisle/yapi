package rules

import (
	"context"
	"errors"
	"sort"
	"sync"
)

// ErrRuleNotFound indicates a rule lookup failed.
var ErrRuleNotFound = errors.New("rule not found")

// Store 抽象出规则数据的读取与持久化。
type Store interface {
	List(ctx context.Context) ([]Rule, error)
	Get(ctx context.Context, id string) (Rule, error)
	Save(ctx context.Context, rule Rule) error
	Delete(ctx context.Context, id string) error
}

// MemoryStore 基于内存的简单实现，便于本地开发与测试。
type MemoryStore struct {
	mu    sync.RWMutex
	rules map[string]Rule
}

// NewMemoryStore 初始化一个空的 MemoryStore。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		rules: make(map[string]Rule),
	}
}

// List 返回所有启用和未启用的规则，按优先级降序排序。
func (s *MemoryStore) List(_ context.Context) ([]Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Rule, 0, len(s.rules))
	for _, rule := range s.rules {
		result = append(result, rule)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})
	return result, nil
}

// Get 根据ID查找规则。
func (s *MemoryStore) Get(_ context.Context, id string) (Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rule, ok := s.rules[id]
	if !ok {
		return Rule{}, ErrRuleNotFound
	}
	return rule, nil
}

// Save 新增或更新规则。
func (s *MemoryStore) Save(_ context.Context, rule Rule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[rule.ID] = rule
	return nil
}

// Delete 按ID删除规则。
func (s *MemoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.rules[id]; !exists {
		return ErrRuleNotFound
	}
	delete(s.rules, id)
	return nil
}
