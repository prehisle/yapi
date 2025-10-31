package rules

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DBStore 基于关系型数据库（PostgreSQL）的规则存储。
type DBStore struct {
	db *gorm.DB
}

// NewDBStore 使用给定 gorm.DB 初始化存储。
func NewDBStore(db *gorm.DB) *DBStore {
	return &DBStore{db: db}
}

// AutoMigrate 执行规则表结构迁移。
func (s *DBStore) AutoMigrate(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(&ruleRecord{})
}

// List 查询所有规则，按优先级降序排列。
func (s *DBStore) List(ctx context.Context) ([]Rule, error) {
	var records []ruleRecord
	if err := s.db.WithContext(ctx).Order("priority DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]Rule, 0, len(records))
	for _, rec := range records {
		rule, err := rec.toDomain()
		if err != nil {
			return nil, err
		}
		result = append(result, rule)
	}
	return result, nil
}

// Get 根据 ID 查询规则。
func (s *DBStore) Get(ctx context.Context, id string) (Rule, error) {
	var rec ruleRecord
	err := s.db.WithContext(ctx).First(&rec, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Rule{}, ErrRuleNotFound
	}
	if err != nil {
		return Rule{}, err
	}
	return rec.toDomain()
}

// Save 插入或更新规则。
func (s *DBStore) Save(ctx context.Context, rule Rule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	rec, err := newRuleRecord(rule)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&rec).Error
}

// Delete 删除指定规则。
func (s *DBStore) Delete(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).Delete(&ruleRecord{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRuleNotFound
	}
	return nil
}

type ruleRecord struct {
	ID        string         `gorm:"primaryKey;type:varchar(64)"`
	Priority  int            `gorm:"index"`
	Matcher   datatypes.JSON `gorm:"type:jsonb"`
	Actions   datatypes.JSON `gorm:"type:jsonb"`
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func newRuleRecord(rule Rule) (ruleRecord, error) {
	matcherJSON, err := json.Marshal(rule.Matcher)
	if err != nil {
		return ruleRecord{}, err
	}
	actionsJSON, err := json.Marshal(rule.Actions)
	if err != nil {
		return ruleRecord{}, err
	}
	return ruleRecord{
		ID:       rule.ID,
		Priority: rule.Priority,
		Matcher:  datatypes.JSON(matcherJSON),
		Actions:  datatypes.JSON(actionsJSON),
		Enabled:  rule.Enabled,
	}, nil
}

func (r ruleRecord) toDomain() (Rule, error) {
	var matcher Matcher
	if err := json.Unmarshal([]byte(r.Matcher), &matcher); err != nil {
		return Rule{}, err
	}
	var actions Actions
	if err := json.Unmarshal([]byte(r.Actions), &actions); err != nil {
		return Rule{}, err
	}
	return Rule{
		ID:       r.ID,
		Priority: r.Priority,
		Matcher:  matcher,
		Actions:  actions,
		Enabled:  r.Enabled,
	}, nil
}
