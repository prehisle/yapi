import { useCallback, useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuth } from '../hooks/useAuth'
import { apiClient, UnauthorizedError } from '../lib/api'
import type { Rule } from '../types/rule'
import { useUIContext } from '../context/UIContext'

const stringifyRecord = (record?: Record<string, string>) => {
  if (!record) return ''
  return Object.entries(record)
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

const stringifyJSON = (record?: Record<string, unknown>) => {
  if (!record) return ''
  try {
    return JSON.stringify(record, null, 2)
  } catch {
    return ''
  }
}

const parseKeyValueBlock = (input: string) => {
  if (!input.trim()) return undefined
  const result: Record<string, string> = {}
  const lines = input.split(/\n+/)
  for (const raw of lines) {
    const line = raw.trim()
    if (!line) continue
    const [key, value] = line.split('=')
    if (!key || value === undefined) {
      throw new Error('头部配置格式应为 key=value，每行一条')
    }
    result[key.trim()] = value.trim()
  }
  return Object.keys(result).length > 0 ? result : undefined
}

const parseList = (input: string) => {
  if (!input.trim()) return undefined
  const items = input
    .split(/[,\n]/)
    .map((item) => item.trim())
    .filter(Boolean)
  return items.length > 0 ? items : undefined
}

const parseJSONBlock = (input: string) => {
  if (!input.trim()) return undefined
  try {
    const value = JSON.parse(input)
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      return value as Record<string, unknown>
    }
    throw new Error('override_json 需为对象类型 JSON')
  } catch (err) {
    throw new Error(`override_json 解析失败: ${(err as Error).message}`)
  }
}

type RuleFormState = {
  id: string
  priority: number | ''
  pathPrefix: string
  methods: string
  targetURL: string
  enabled: boolean
  setHeaders: string
  addHeaders: string
  removeHeaders: string
  setAuthorization: string
  overrideJSON: string
  removeJSON: string
}

const emptyFormState: RuleFormState = {
  id: '',
  priority: '',
  pathPrefix: '',
  methods: '',
  targetURL: '',
  enabled: true,
  setHeaders: '',
  addHeaders: '',
  removeHeaders: '',
  setAuthorization: '',
  overrideJSON: '',
  removeJSON: '',
}

const RulesPage = () => {
  const { logout } = useAuth()
  const { showSuccess, showError, confirm } = useUIContext()
  const navigate = useNavigate()
  const [rules, setRules] = useState<Rule[]>([])
  const [loading, setLoading] = useState<boolean>(true)
  const [error, setError] = useState<string | null>(null)
  const [updatingId, setUpdatingId] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [formOpen, setFormOpen] = useState(false)
  const [formLoading, setFormLoading] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [formState, setFormState] = useState<RuleFormState>(emptyFormState)
  const [editingRuleId, setEditingRuleId] = useState<string | null>(null)

  const fetchRules = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await apiClient.get<Rule[]>('/admin/rules')
      setRules(data)
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        logout()
        navigate('/login', { replace: true })
        return
      }
      const message = (err as Error).message ?? '拉取规则失败'
      setError(message)
      showError(message)
    } finally {
      setLoading(false)
    }
  }, [logout, navigate, showError])

  useEffect(() => {
    void fetchRules()
  }, [fetchRules])

  const filteredRules = useMemo(() => {
    if (!search.trim()) {
      return rules
    }
    const keyword = search.trim().toLowerCase()
    return rules.filter((rule) => {
      return (
        rule.id.toLowerCase().includes(keyword) ||
        (rule.matcher.path_prefix ?? '').toLowerCase().includes(keyword) ||
        (rule.actions.set_target_url ?? '').toLowerCase().includes(keyword)
      )
    })
  }, [rules, search])

  const totalPages = useMemo(() => {
    return Math.max(1, Math.ceil(filteredRules.length / pageSize))
  }, [filteredRules.length, pageSize])

  useEffect(() => {
    setPage(1)
  }, [search, pageSize])

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages)
    }
  }, [page, totalPages])

  const paginatedRules = useMemo(() => {
    const start = (page - 1) * pageSize
    return filteredRules.slice(start, start + pageSize)
  }, [filteredRules, page, pageSize])

  const activeCount = useMemo(
    () => rules.filter((rule) => rule.enabled).length,
    [rules],
  )

  const handleToggleRule = useCallback(
    async (rule: Rule) => {
      setUpdatingId(rule.id)
      setError(null)
      try {
        const payload: Rule = {
          ...rule,
          enabled: !rule.enabled,
        }
        await apiClient.put(`/admin/rules/${rule.id}`, payload)
        await fetchRules()
        showSuccess(`规则 ${rule.id} 已${rule.enabled ? '禁用' : '启用'}`)
      } catch (err) {
        if (err instanceof UnauthorizedError) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        const message = (err as Error).message ?? '更新规则失败'
        setError(message)
        showError(message)
      } finally {
        setUpdatingId(null)
      }
    },
    [fetchRules, logout, navigate, showError, showSuccess],
  )

  const openCreateDialog = useCallback(() => {
    setEditingRuleId(null)
    setFormState(emptyFormState)
    setFormError(null)
    setFormOpen(true)
  }, [])

  const openEditDialog = useCallback((rule: Rule) => {
    setEditingRuleId(rule.id)
    setFormState({
      id: rule.id,
      priority: rule.priority,
      pathPrefix: rule.matcher.path_prefix ?? '',
      methods: rule.matcher.methods?.join(', ') ?? '',
      targetURL: rule.actions.set_target_url ?? '',
      enabled: rule.enabled,
      setHeaders: stringifyRecord(rule.actions.set_headers),
      addHeaders: stringifyRecord(rule.actions.add_headers),
      removeHeaders: rule.actions.remove_headers?.join(', ') ?? '',
      setAuthorization: rule.actions.set_authorization ?? '',
      overrideJSON: stringifyJSON(rule.actions.override_json),
      removeJSON: rule.actions.remove_json?.join(', ') ?? '',
    })
    setFormError(null)
    setFormOpen(true)
  }, [])

  const handleDeleteRule = useCallback(
    (rule: Rule) => {
      confirm({
        title: '删除规则',
        message: `确认删除规则 ${rule.id} 吗？该操作不可撤销。`,
        confirmText: '删除',
        onConfirm: async () => {
          setUpdatingId(rule.id)
          try {
            await apiClient.delete(`/admin/rules/${rule.id}`)
            await fetchRules()
            showSuccess(`规则 ${rule.id} 已删除`)
          } catch (err) {
            if (err instanceof UnauthorizedError) {
              logout()
              navigate('/login', { replace: true })
              return
            }
            showError((err as Error).message ?? '删除规则失败')
          } finally {
            setUpdatingId(null)
          }
        },
      })
    },
    [confirm, fetchRules, logout, navigate, showError, showSuccess],
  )

  const closeForm = useCallback(() => {
    if (formLoading) return
    setFormOpen(false)
    setFormState(emptyFormState)
    setEditingRuleId(null)
    setFormError(null)
  }, [formLoading])

  const handleFormSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!formState.id.trim()) {
        setFormError('规则 ID 必填')
        return
      }
      if (formState.priority === '') {
        setFormError('请填写优先级')
        return
      }
      let parsedSetHeaders: Record<string, string> | undefined
      let parsedAddHeaders: Record<string, string> | undefined
      let parsedRemoveHeaders: string[] | undefined
      let parsedOverrideJSON: Record<string, unknown> | undefined
      let parsedRemoveJSON: string[] | undefined

      try {
        parsedSetHeaders = parseKeyValueBlock(formState.setHeaders)
        parsedAddHeaders = parseKeyValueBlock(formState.addHeaders)
        parsedRemoveHeaders = parseList(formState.removeHeaders)
        parsedOverrideJSON = parseJSONBlock(formState.overrideJSON)
        parsedRemoveJSON = parseList(formState.removeJSON)
      } catch (err) {
        setFormError((err as Error).message)
        return
      }

      setFormLoading(true)
      setFormError(null)
      const payload: Rule = {
        id: formState.id.trim(),
        priority: Number(formState.priority),
        enabled: formState.enabled,
        matcher: {
          path_prefix: formState.pathPrefix || undefined,
          methods: formState.methods
            ? formState.methods
                .split(',')
                .map((item) => item.trim())
                .filter(Boolean)
            : undefined,
        },
        actions: {
          set_target_url: formState.targetURL || undefined,
          set_headers: parsedSetHeaders,
          add_headers: parsedAddHeaders,
          remove_headers: parsedRemoveHeaders,
          set_authorization: formState.setAuthorization || undefined,
          override_json: parsedOverrideJSON,
          remove_json: parsedRemoveJSON,
        },
      }
      try {
        if (editingRuleId) {
          await apiClient.put(`/admin/rules/${editingRuleId}`, payload)
        } else {
          await apiClient.post('/admin/rules', payload)
        }
        await fetchRules()
        closeForm()
        showSuccess(`规则 ${payload.id} 已保存`)
      } catch (err) {
        if (err instanceof UnauthorizedError) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        const message = (err as Error).message ?? '提交失败'
        setFormError(message)
        showError(message)
      } finally {
        setFormLoading(false)
      }
    },
    [closeForm, fetchRules, formState, editingRuleId, logout, navigate, showError, showSuccess],
  )

  return (
    <div className="page">
      <header className="page__header">
        <div>
          <h1>规则管理</h1>
          <p style={{ margin: '8px 0 0', color: '#475467' }}>
            共 {rules.length} 条规则，启用 {activeCount} 条
          </p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <button className="button" onClick={openCreateDialog}>
            新建规则
          </button>
          <button className="button button--ghost" onClick={logout}>
            退出登录
          </button>
          <button className="button" onClick={() => fetchRules()} disabled={loading}>
            {loading ? '刷新中...' : '刷新'}
          </button>
        </div>
      </header>

      <div className="toolbar">
        <input
          className="field__input field__input--inline"
          placeholder="搜索 ID、路径或目标地址"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <select
          className="field__input field__input--inline"
          value={pageSize}
          onChange={(event) => setPageSize(Number(event.target.value))}
        >
          {[10, 20, 50].map((size) => (
            <option key={size} value={size}>
              每页 {size} 条
            </option>
          ))}
        </select>
      </div>

      {error ? <div className="alert alert--error">{error}</div> : null}

      {loading ? (
        <p>加载中...</p>
      ) : filteredRules.length === 0 ? (
        <div className="notice">暂无规则，点击“新建规则”开始配置。</div>
      ) : (
        <div className="table-wrapper">
          <table className="table">
            <thead>
              <tr>
                <th>ID</th>
                <th>优先级</th>
                <th>路径前缀</th>
                <th>方法</th>
                <th>目标地址</th>
                <th>状态</th>
                <th style={{ width: 160 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {paginatedRules.map((rule) => (
                <tr key={rule.id}>
                  <td>{rule.id}</td>
                  <td>{rule.priority}</td>
                  <td>{rule.matcher.path_prefix ?? '-'}</td>
                  <td>{rule.matcher.methods?.join(', ') ?? 'ALL'}</td>
                  <td>{rule.actions.set_target_url ?? '-'}</td>
                  <td>
                    <span
                      className={rule.enabled ? 'status status--active' : 'status'}
                    >
                      {rule.enabled ? '已启用' : '未启用'}
                    </span>
                  </td>
                  <td>
                    <div className="table-actions">
                      <button
                        className="button button--ghost"
                        onClick={() => handleToggleRule(rule)}
                        disabled={updatingId === rule.id}
                      >
                        {rule.enabled ? '禁用' : '启用'}
                      </button>
                      <button
                        className="button button--ghost"
                        onClick={() => openEditDialog(rule)}
                      >
                        编辑
                      </button>
                      <button
                        className="button button--ghost"
                        onClick={() => handleDeleteRule(rule)}
                        disabled={updatingId === rule.id}
                      >
                        删除
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {filteredRules.length > 0 ? (
        <div className="pagination">
          <button
            className="button button--ghost"
            onClick={() => setPage((prev) => Math.max(1, prev - 1))}
            disabled={page === 1}
          >
            上一页
          </button>
          <span>
            第 {page} / {totalPages} 页
          </span>
          <button
            className="button button--ghost"
            onClick={() => setPage((prev) => Math.min(totalPages, prev + 1))}
            disabled={page === totalPages}
          >
            下一页
          </button>
        </div>
      ) : null}

      {formOpen ? (
        <div className="dialog-backdrop" role="dialog" aria-modal>
          <form className="dialog" onSubmit={handleFormSubmit}>
            <h2 className="dialog__title">
              {editingRuleId ? '编辑规则' : '新建规则'}
            </h2>
            <label className="field">
              <span className="field__label">规则 ID</span>
              <input
                className="field__input"
                value={formState.id}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, id: event.target.value }))
                }
                required
                disabled={Boolean(editingRuleId)}
              />
            </label>
            <label className="field">
              <span className="field__label">优先级</span>
              <input
                className="field__input"
                type="number"
                value={formState.priority}
                onChange={(event) =>
          setFormState((prev) => ({
            ...prev,
            priority: event.target.value === '' ? '' : Number(event.target.value),
          }))
                }
                required
              />
            </label>
            <label className="field">
              <span className="field__label">路径前缀</span>
              <input
                className="field__input"
                value={formState.pathPrefix}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, pathPrefix: event.target.value }))
                }
                placeholder="例如 /v1/chat"
              />
            </label>
            <label className="field">
              <span className="field__label">方法列表</span>
              <input
                className="field__input"
                value={formState.methods}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, methods: event.target.value }))
                }
                placeholder="逗号分隔，例如 POST,GET"
              />
              <span className="field__hint">留空表示匹配所有方法。</span>
            </label>
            <label className="field">
              <span className="field__label">目标地址</span>
              <input
                className="field__input"
                value={formState.targetURL}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, targetURL: event.target.value }))
                }
                placeholder="https://api.openai.com"
              />
            </label>
            <label className="field">
              <span className="field__label">覆盖头部（set_headers）</span>
              <textarea
                className="field__textarea"
                placeholder="key=value，每行一条"
                value={formState.setHeaders}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, setHeaders: event.target.value }))
                }
                rows={3}
              />
              <span className="field__hint">逐行填写 key=value，将覆盖现有头部。</span>
            </label>
            <label className="field">
              <span className="field__label">追加头部（add_headers）</span>
              <textarea
                className="field__textarea"
                placeholder="key=value，每行一条"
                value={formState.addHeaders}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, addHeaders: event.target.value }))
                }
                rows={3}
              />
              <span className="field__hint">逐行填写 key=value，将在原有头部基础上追加。</span>
            </label>
            <label className="field">
              <span className="field__label">移除头部（remove_headers）</span>
              <textarea
                className="field__textarea"
                placeholder="header1, header2"
                value={formState.removeHeaders}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, removeHeaders: event.target.value }))
                }
                rows={2}
              />
              <span className="field__hint">可用逗号或换行分隔多个头部。</span>
            </label>
            <label className="field">
              <span className="field__label">Authorization 头</span>
              <input
                className="field__input"
                value={formState.setAuthorization}
                onChange={(event) =>
                  setFormState((prev) => ({
                    ...prev,
                    setAuthorization: event.target.value,
                  }))
                }
                placeholder="Bearer xxx"
              />
            </label>
            <label className="field">
              <span className="field__label">override_json</span>
              <textarea
                className="field__textarea"
                placeholder={`{\n  "model": "gpt-4.1"\n}`}
                value={formState.overrideJSON}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, overrideJSON: event.target.value }))
                }
                rows={4}
              />
              <span className="field__hint">需填写 JSON 对象，留空表示不修改。</span>
            </label>
            <label className="field">
              <span className="field__label">remove_json</span>
              <textarea
                className="field__textarea"
                placeholder="messages[0].content, metadata.trace_id"
                value={formState.removeJSON}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, removeJSON: event.target.value }))
                }
                rows={2}
              />
              <span className="field__hint">支持点号与数组下标语法，留空表示不移除。</span>
            </label>
            <label className="field field--inline">
              <input
                type="checkbox"
                checked={formState.enabled}
                onChange={(event) =>
                  setFormState((prev) => ({ ...prev, enabled: event.target.checked }))
                }
              />
              <span className="field__label">启用规则</span>
            </label>
            {formError ? <p className="field__error">{formError}</p> : null}
            <div className="dialog__actions">
              <button
                type="button"
                className="button button--ghost"
                onClick={closeForm}
                disabled={formLoading}
              >
                取消
              </button>
              <button className="button" type="submit" disabled={formLoading}>
                {formLoading ? '提交中...' : '保存'}
              </button>
            </div>
          </form>
        </div>
      ) : null}
    </div>
  )
}

export default RulesPage
