package rules

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidJSONPath 表示提供的 JSON 路径不合法。
var ErrInvalidJSONPath = errors.New("invalid json path")

// JSONPathToken 描述 JSON 路径上的一个节点，可能是键或数组索引。
type JSONPathToken struct {
	Key   string
	Index *int
}

// IsKey 判断当前节点是否为对象键。
func (t JSONPathToken) IsKey() bool {
	return t.Index == nil
}

// IsIndex 判断当前节点是否为数组索引。
func (t JSONPathToken) IsIndex() bool {
	return t.Index != nil
}

// IndexValue 返回索引的整数值，如果当前节点不是索引会 panic。
func (t JSONPathToken) IndexValue() int {
	if t.Index == nil {
		panic("JSONPathToken.IndexValue called on key token")
	}
	return *t.Index
}

// ParseJSONPath 将形如 `metadata.trace_id` 或 `messages[0].role` 的路径解析为节点切片。
func ParseJSONPath(path string) ([]JSONPathToken, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("%w: empty path", ErrInvalidJSONPath)
	}
	var tokens []JSONPathToken
	var segment strings.Builder
	for i := 0; i < len(path); {
		switch path[i] {
		case '.':
			if segment.Len() == 0 {
				if i > 0 && path[i-1] == ']' {
					i++
					continue
				}
				return nil, fmt.Errorf("%w: empty segment in %q", ErrInvalidJSONPath, path)
			}
			token, err := buildToken(segment.String())
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token)
			segment.Reset()
			i++
		case '[':
			if segment.Len() > 0 {
				token, err := buildToken(segment.String())
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, token)
				segment.Reset()
			}
			i++
			start := i
			for i < len(path) && path[i] != ']' {
				if path[i] < '0' || path[i] > '9' {
					return nil, fmt.Errorf("%w: non-digit array index in %q", ErrInvalidJSONPath, path)
				}
				i++
			}
			if i >= len(path) {
				return nil, fmt.Errorf("%w: missing closing bracket in %q", ErrInvalidJSONPath, path)
			}
			if start == i {
				return nil, fmt.Errorf("%w: empty array index in %q", ErrInvalidJSONPath, path)
			}
			idx, err := strconv.Atoi(path[start:i])
			if err != nil {
				return nil, fmt.Errorf("%w: invalid array index in %q: %v", ErrInvalidJSONPath, path, err)
			}
			tokens = append(tokens, JSONPathToken{Index: &idx})
			i++ // Skip ']'
		default:
			segment.WriteByte(path[i])
			i++
		}
	}
	if segment.Len() > 0 {
		token, err := buildToken(segment.String())
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("%w: empty path", ErrInvalidJSONPath)
	}
	return tokens, nil
}

func buildToken(segment string) (JSONPathToken, error) {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return JSONPathToken{}, fmt.Errorf("%w: empty segment", ErrInvalidJSONPath)
	}
	if idx, err := strconv.Atoi(segment); err == nil {
		return JSONPathToken{Index: &idx}, nil
	}
	return JSONPathToken{Key: segment}, nil
}
