package rules

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ErrInvalidRule signals that a rule failed basic validation.
var ErrInvalidRule = errors.New("invalid rule")

// Rule 定义了一条完整的代理规则。
type Rule struct {
	ID        string    `json:"id"`
	Priority  int       `json:"priority"`
	Matcher   Matcher   `json:"matcher"`
	Actions   Actions   `json:"actions"`
	Enabled   bool      `json:"enabled"`
	Version   int       `json:"version"`
	CreatedBy string    `json:"created_by,omitempty"`
	UpdatedBy string    `json:"updated_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Matcher 描述了匹配客户端请求的条件。
type Matcher struct {
	PathPrefix string            `json:"path_prefix,omitempty"`
	Methods    []string          `json:"methods,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// Actions 表示命中的规则执行的操作。
type Actions struct {
	SetTargetURL     string                 `json:"set_target_url,omitempty"`
	SetHeaders       map[string]string      `json:"set_headers,omitempty"`
	AddHeaders       map[string]string      `json:"add_headers,omitempty"`
	RemoveHeaders    []string               `json:"remove_headers,omitempty"`
	SetAuthorization string                 `json:"set_authorization,omitempty"`
	OverrideJSON     map[string]any         `json:"override_json,omitempty"`
	RemoveJSON       []string               `json:"remove_json,omitempty"`
	RewritePathRegex *RewritePathExpression `json:"rewrite_path_regex,omitempty"`
	Script           string                 `json:"script,omitempty"`
}

// RewritePathExpression 封装重写路径所需的正则参数。
type RewritePathExpression struct {
	Pattern string `json:"pattern"`
	Replace string `json:"replace"`
}

// Validate 检查规则定义是否符合要求。
func (r Rule) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrInvalidRule)
	}
	if r.Matcher.PathPrefix == "" && len(r.Matcher.Methods) == 0 && len(r.Matcher.Headers) == 0 {
		return fmt.Errorf("%w: matcher must not be empty", ErrInvalidRule)
	}
	if err := validateMatcher(r.Matcher); err != nil {
		return err
	}
	if err := validateActions(r.Actions); err != nil {
		return err
	}
	return nil
}

func validateMatcher(m Matcher) error {
	if m.PathPrefix != "" && !strings.HasPrefix(m.PathPrefix, "/") {
		return fmt.Errorf("%w: path_prefix must start with '/'", ErrInvalidRule)
	}
	for i, method := range m.Methods {
		if strings.TrimSpace(method) == "" {
			return fmt.Errorf("%w: methods[%d] must not be empty", ErrInvalidRule, i)
		}
	}
	for key := range m.Headers {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: header key must not be empty", ErrInvalidRule)
		}
	}
	return nil
}

func validateActions(a Actions) error {
	if a.SetTargetURL == "" && len(a.SetHeaders) == 0 &&
		len(a.AddHeaders) == 0 && len(a.RemoveHeaders) == 0 &&
		strings.TrimSpace(a.SetAuthorization) == "" &&
		len(a.OverrideJSON) == 0 && len(a.RemoveJSON) == 0 &&
		a.RewritePathRegex == nil && strings.TrimSpace(a.Script) == "" {
		return fmt.Errorf("%w: actions must not be empty", ErrInvalidRule)
	}
	if a.RewritePathRegex != nil {
		if _, err := regexp.Compile(a.RewritePathRegex.Pattern); err != nil {
			return fmt.Errorf("%w: invalid rewrite regex pattern: %v", ErrInvalidRule, err)
		}
	}
	for key := range a.OverrideJSON {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: override_json key must not be empty", ErrInvalidRule)
		}
		if _, err := ParseJSONPath(key); err != nil {
			return fmt.Errorf("%w: override_json path %q invalid: %v", ErrInvalidRule, key, err)
		}
	}
	for i, key := range a.RemoveJSON {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: remove_json[%d] must not be empty", ErrInvalidRule, i)
		}
		if _, err := ParseJSONPath(key); err != nil {
			return fmt.Errorf("%w: remove_json path %q invalid: %v", ErrInvalidRule, key, err)
		}
	}
	return nil
}
