import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { RuleFormDialog } from '../components/RuleFormDialog'
import { RuleDetailDrawer } from '../components/RuleDetailDrawer'
import { Button } from '../components/ui/Button'
import { SearchInput } from '../components/ui/SearchInput'
import { Select } from '../components/ui/Select'
import { useAuth } from '../hooks/useAuth'
import { useUIContext } from '../context/UIContext'
import { apiClient, UnauthorizedError } from '../lib/api'
import type { Rule, RuleListResponse } from '../types/rule'

type DialogState =
  | { mode: 'create'; rule?: undefined }
  | { mode: 'edit'; rule: Rule }
  | null

type StatusFilter = 'all' | 'enabled' | 'disabled'

const RulesPage = () => {
  const { logout } = useAuth()
  const { showSuccess, showError, confirm } = useUIContext()
  const navigate = useNavigate()

  const [rules, setRules] = useState<Rule[]>([])
  const [total, setTotal] = useState(0)
  const [enabledTotal, setEnabledTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [updatingId, setUpdatingId] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const [dialogState, setDialogState] = useState<DialogState>(null)
  const [detailRule, setDetailRule] = useState<Rule | null>(null)

  const fetchRules = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams()
      params.set('page', String(page))
      params.set('page_size', String(pageSize))
      const keyword = search.trim()
      if (keyword) {
        params.set('q', keyword)
      }
      if (statusFilter !== 'all') {
        params.set('enabled', statusFilter === 'enabled' ? 'true' : 'false')
      }
      const query = params.toString()
      const path = query ? `/admin/rules?${query}` : '/admin/rules'
      const data = await apiClient.get<RuleListResponse>(path)
      setRules(data.items)
      setTotal(data.total)
      setEnabledTotal(data.enabled_total)
      if (data.page !== page) {
        setPage(data.page)
      }
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
  }, [page, pageSize, search, statusFilter, logout, navigate, showError])

  useEffect(() => {
    void fetchRules()
  }, [fetchRules])

  useEffect(() => {
    setPage((prev) => (prev === 1 ? prev : 1))
  }, [search, pageSize, statusFilter])

  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / pageSize)), [total, pageSize])

  const isEmpty = !loading && rules.length === 0

  const openCreateDialog = useCallback(() => {
    setDialogState({ mode: 'create' })
  }, [])

  const openEditDialog = useCallback((rule: Rule) => {
    setDialogState({ mode: 'edit', rule })
  }, [])

  const closeDialog = useCallback(() => {
    setDialogState(null)
  }, [])

  const openDetail = useCallback((rule: Rule) => {
    setDetailRule(rule)
  }, [])

  const closeDetail = useCallback(() => {
    setDetailRule(null)
  }, [])

  const handleSaveRule = useCallback(
    async (payload: Rule) => {
      try {
        if (dialogState?.mode === 'edit') {
          await apiClient.put(`/admin/rules/${dialogState.rule.id}`, payload)
        } else {
          await apiClient.post('/admin/rules', payload)
        }
        await fetchRules()
        showSuccess(`规则 ${payload.id} 已保存`)
        setDialogState(null)
      } catch (err) {
        if (err instanceof UnauthorizedError) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        throw err
      }
    },
    [dialogState, fetchRules, logout, navigate, showSuccess],
  )

  const handleToggleRule = useCallback(
    async (rule: Rule) => {
      setUpdatingId(rule.id)
      try {
        const payload: Rule = { ...rule, enabled: !rule.enabled }
        await apiClient.put(`/admin/rules/${rule.id}`, payload)
        await fetchRules()
        showSuccess(`规则 ${rule.id} 已${rule.enabled ? '禁用' : '启用'}`)
      } catch (err) {
        if (err instanceof UnauthorizedError) {
          logout()
          navigate('/login', { replace: true })
          return
        }
        showError((err as Error).message ?? '更新规则失败')
      } finally {
        setUpdatingId(null)
      }
    },
    [fetchRules, logout, navigate, showError, showSuccess],
  )

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

  const handleRefresh = useCallback(() => {
    void fetchRules()
  }, [fetchRules])

  return (
    <div className="page">
      <header className="page__header">
        <div>
          <h1 className="page__title">规则管理</h1>
          <p className="page__description">
            共 {total} 条规则，启用 {enabledTotal} 条
          </p>
        </div>
        <div className="flex gap-3">
          <Button onClick={openCreateDialog} variant="primary">
            新建规则
          </Button>
          <Button onClick={handleRefresh} variant="ghost" disabled={loading}>
            {loading ? '刷新中...' : '刷新'}
          </Button>
        </div>
      </header>

      <div className="toolbar">
        <div className="toolbar__group">
          <SearchInput
            placeholder="搜索 ID、路径或目标地址"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            onClear={() => setSearch('')}
          />

          <Select
            value={statusFilter}
            onChange={(event) => setStatusFilter(event.target.value as StatusFilter)}
            options={[
              { value: 'all', label: '全部状态' },
              { value: 'enabled', label: '仅启用' },
              { value: 'disabled', label: '仅未启用' },
            ]}
          />

          <Select
            value={String(pageSize)}
            onChange={(event) => setPageSize(Number(event.target.value))}
            options={[10, 20, 50].map((size) => ({
              value: String(size),
              label: `每页 ${size} 条`,
            }))}
          />
        </div>
      </div>

      {error ? <div className="alert alert--error">{error}</div> : null}

      {loading ? (
        <p>加载中...</p>
      ) : isEmpty ? (
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
                <th style={{ width: 200 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {rules.map((rule) => (
                <tr key={rule.id}>
                  <td>{rule.id}</td>
                  <td>{rule.priority}</td>
                  <td>{rule.matcher.path_prefix ?? '-'}</td>
                  <td>{rule.matcher.methods?.join(', ') ?? 'ALL'}</td>
                  <td>{rule.actions.set_target_url ?? '-'}</td>
                  <td>
                    <span className={rule.enabled ? 'status status--active' : 'status'}>
                      {rule.enabled ? '已启用' : '未启用'}
                    </span>
                  </td>
                  <td>
                    <div className="table-actions">
                      <Button variant="ghost" size="sm" onClick={() => openDetail(rule)}>
                        详情
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleToggleRule(rule)}
                        disabled={updatingId === rule.id}
                      >
                        {rule.enabled ? '禁用' : '启用'}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => openEditDialog(rule)}>
                        编辑
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDeleteRule(rule)}
                        disabled={updatingId === rule.id}
                      >
                        删除
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {total > 0 ? (
        <div className="pagination">
          <div className="pagination__info">
            第 {page} / {totalPages} 页，共 {total} 条记录
          </div>
          <div className="pagination__controls">
            <Button
              variant="ghost"
              onClick={() => setPage((prev) => Math.max(1, prev - 1))}
              disabled={page === 1 || loading}
              size="sm"
            >
              上一页
            </Button>
            <Button
              variant="ghost"
              onClick={() => setPage((prev) => Math.min(totalPages, prev + 1))}
              disabled={page >= totalPages || loading}
              size="sm"
            >
              下一页
            </Button>
          </div>
        </div>
      ) : null}

      <RuleFormDialog
        open={dialogState !== null}
        mode={dialogState?.mode ?? 'create'}
        initialRule={dialogState?.mode === 'edit' ? dialogState.rule : undefined}
        onClose={closeDialog}
        onSubmit={handleSaveRule}
      />
      <RuleDetailDrawer open={detailRule !== null} rule={detailRule ?? undefined} onClose={closeDetail} />
    </div>
  )
}

export default RulesPage
