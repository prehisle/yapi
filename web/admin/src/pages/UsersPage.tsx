import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { ApiError, UnauthorizedError, apiClient } from '../lib/api'
import { useAuth } from '../hooks/useAuth'
import { useUIContext } from '../context/UIContext'
import type {
  APIKey,
  APIKeyBinding,
  APIKeyCreateResponse,
  APIKeyListResponse,
  UpstreamCredential,
  UpstreamCredentialListResponse,
  User,
  UserListResponse,
} from '../types/accounts'

type DialogState =
  | { type: 'create-user' }
  | { type: 'create-api-key' }
  | { type: 'create-upstream' }
  | null

const emptyUserForm = { name: '', description: '' }
const emptyUpstreamForm = { provider: '', label: '', plaintext: '', endpoints: '', metadata: '' }

const UsersPage = () => {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const { showSuccess, showError, confirm } = useUIContext()

  const [users, setUsers] = useState<User[]>([])
  const [loadingUsers, setLoadingUsers] = useState(true)
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null)

  const [apiKeys, setApiKeys] = useState<APIKey[]>([])
  const [bindings, setBindings] = useState<Record<string, APIKeyBinding | null>>({})
  const [upstreams, setUpstreams] = useState<UpstreamCredential[]>([])
  const [loadingDetails, setLoadingDetails] = useState(false)
  const [apiKeySecret, setApiKeySecret] = useState<string | null>(null)

  const [dialog, setDialog] = useState<DialogState>(null)
  const [userForm, setUserForm] = useState(emptyUserForm)
  const [upstreamForm, setUpstreamForm] = useState(emptyUpstreamForm)

  const selectedUser = useMemo(
    () => users.find((user) => user.id === selectedUserId) ?? null,
    [users, selectedUserId],
  )

  const handleUnauthorized = useCallback(() => {
    logout()
    navigate('/login', { replace: true })
  }, [logout, navigate])

  const loadUsers = useCallback(async () => {
    setLoadingUsers(true)
    try {
      const response = await apiClient.get<UserListResponse>('/admin/users')
      setUsers(response.items)
      if (!selectedUserId && response.items.length > 0) {
        setSelectedUserId(response.items[0].id)
      }
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        handleUnauthorized()
        return
      }
      showError((err as Error).message ?? '拉取用户失败')
    } finally {
      setLoadingUsers(false)
    }
  }, [selectedUserId, handleUnauthorized, showError])

  const loadBindingsForKeys = useCallback(
    async (keys: APIKey[]) => {
      const nextBindings: Record<string, APIKeyBinding | null> = {}
      await Promise.all(
        keys.map(async (key) => {
          try {
            const binding = await apiClient.get<APIKeyBinding>(`/admin/api-keys/${key.id}/binding`)
            nextBindings[key.id] = binding
          } catch (err) {
            if (err instanceof UnauthorizedError) {
              handleUnauthorized()
              return
            }
            if (err instanceof ApiError && err.status === 404) {
              nextBindings[key.id] = null
              return
            }
            showError((err as Error).message ?? '加载密钥绑定失败')
          }
        }),
      )
      setBindings(nextBindings)
    },
    [handleUnauthorized, showError],
  )

  const loadUserDetails = useCallback(
    async (userId: string) => {
      setLoadingDetails(true)
      try {
        const [keyResp, upstreamResp] = await Promise.all([
          apiClient.get<APIKeyListResponse>(`/admin/users/${userId}/api-keys`),
          apiClient.get<UpstreamCredentialListResponse>(`/admin/users/${userId}/upstreams`),
        ])
        setApiKeys(keyResp.items)
        setUpstreams(upstreamResp.items)
        await loadBindingsForKeys(keyResp.items)
      } catch (err) {
        if (err instanceof UnauthorizedError) {
          handleUnauthorized()
          return
        }
        showError((err as Error).message ?? '加载用户详情失败')
      } finally {
        setLoadingDetails(false)
      }
    },
    [handleUnauthorized, loadBindingsForKeys, showError],
  )

  useEffect(() => {
    void loadUsers()
  }, [loadUsers])

  useEffect(() => {
    if (selectedUserId) {
      void loadUserDetails(selectedUserId)
    } else {
      setApiKeys([])
      setBindings({})
      setUpstreams([])
    }
  }, [selectedUserId, loadUserDetails])

  const openCreateUserDialog = () => {
    setUserForm(emptyUserForm)
    setDialog({ type: 'create-user' })
  }

  const openCreateAPIKeyDialog = () => {
    setDialog({ type: 'create-api-key' })
    setApiKeySecret(null)
  }

  const openCreateUpstreamDialog = () => {
    setUpstreamForm(emptyUpstreamForm)
    setDialog({ type: 'create-upstream' })
  }

  const closeDialog = () => {
    setDialog(null)
  }

  const handleCreateUser = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    try {
      await apiClient.post<User>('/admin/users', {
        name: userForm.name,
        description: userForm.description,
      })
      showSuccess('用户已创建')
      closeDialog()
      await loadUsers()
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        handleUnauthorized()
        return
      }
      showError((err as Error).message ?? '创建用户失败')
    }
  }

  const handleDeleteUser = (user: User) => {
    confirm({
      title: '删除用户',
      message: `确认删除用户 ${user.name} 吗？该操作不可恢复。`,
      confirmText: '删除',
      onConfirm: async () => {
        try {
          await apiClient.delete(`/admin/users/${user.id}`)
          showSuccess(`用户 ${user.name} 已删除`)
          if (selectedUserId === user.id) {
            setSelectedUserId(null)
          }
          await loadUsers()
        } catch (err) {
          if (err instanceof UnauthorizedError) {
            handleUnauthorized()
            return
          }
          showError((err as Error).message ?? '删除用户失败')
        }
      },
    })
  }

  const handleCreateAPIKey = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!selectedUserId) {
      return
    }
    const formData = new FormData(event.currentTarget)
    const label = (formData.get('label') as string) ?? ''
    try {
      const response = await apiClient.post<APIKeyCreateResponse>(
        `/admin/users/${selectedUserId}/api-keys`,
        { label },
      )
      showSuccess('API Key 已生成')
      setApiKeySecret(response.secret)
      await loadUserDetails(selectedUserId)
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        handleUnauthorized()
        return
      }
      showError((err as Error).message ?? '创建 API Key 失败')
    }
  }

  const handleDeleteAPIKey = (key: APIKey) => {
    confirm({
      title: '撤销 API Key',
      message: `确认撤销密钥 ${key.prefix} 吗？撤销后将无法恢复。`,
      confirmText: '撤销',
      onConfirm: async () => {
        try {
          await apiClient.delete(`/admin/api-keys/${key.id}`)
          showSuccess('API Key 已撤销')
          if (selectedUserId) {
            await loadUserDetails(selectedUserId)
          }
        } catch (err) {
          if (err instanceof UnauthorizedError) {
            handleUnauthorized()
            return
          }
          showError((err as Error).message ?? '撤销 API Key 失败')
        }
      },
    })
  }

  const handleCreateUpstream = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!selectedUserId) {
      return
    }
    try {
      const payload = {
        provider: upstreamForm.provider,
        label: upstreamForm.label,
        plaintext: upstreamForm.plaintext,
        endpoints: upstreamForm.endpoints
          .split(/[\n,]/)
          .map((item) => item.trim())
          .filter(Boolean),
      }
      await apiClient.post<UpstreamCredential>(`/admin/users/${selectedUserId}/upstreams`, payload)
      showSuccess('上游凭据已创建')
      closeDialog()
      await loadUserDetails(selectedUserId)
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        handleUnauthorized()
        return
      }
      showError((err as Error).message ?? '创建上游凭据失败')
    }
  }

  const handleDeleteUpstream = (cred: UpstreamCredential) => {
    confirm({
      title: '删除上游凭据',
      message: `确认删除上游 ${cred.provider} (${cred.label ?? '未命名'}) 吗？`,
      confirmText: '删除',
      onConfirm: async () => {
        try {
          await apiClient.delete(`/admin/upstreams/${cred.id}`)
          showSuccess('上游凭据已删除')
          if (selectedUserId) {
            await loadUserDetails(selectedUserId)
          }
        } catch (err) {
          if (err instanceof UnauthorizedError) {
            handleUnauthorized()
            return
          }
          showError((err as Error).message ?? '删除上游凭据失败')
        }
      },
    })
  }

  const handleBindAPIKey = async (keyId: string, upstreamId: string) => {
    if (!selectedUserId) {
      return
    }
    try {
      const binding = await apiClient.post<APIKeyBinding>(`/admin/api-keys/${keyId}/binding`, {
        user_id: selectedUserId,
        upstream_credential_id: upstreamId,
      })
      setBindings((prev) => ({ ...prev, [keyId]: binding }))
      showSuccess('API Key 已绑定到上游')
    } catch (err) {
      if (err instanceof UnauthorizedError) {
        handleUnauthorized()
        return
      }
      showError((err as Error).message ?? '绑定上游失败')
    }
  }

  return (
    <div className="page">
      <header className="page__header">
        <div>
          <h1>用户管理</h1>
          <p style={{ margin: '8px 0 0', color: '#475467' }}>
            管理调用用户、API Key 以及上游服务凭据
          </p>
        </div>
        <div style={{ display: 'flex', gap: '12px' }}>
          <button className="button" onClick={openCreateUserDialog}>
            新建用户
          </button>
          <button className="button button--ghost" onClick={loadUsers} disabled={loadingUsers}>
            {loadingUsers ? '刷新中...' : '刷新'}
          </button>
        </div>
      </header>

      {loadingUsers ? (
        <p>加载中...</p>
      ) : users.length === 0 ? (
        <div className="notice">暂无用户，点击“新建用户”开始配置。</div>
      ) : (
        <div className="table-wrapper" style={{ marginBottom: 24 }}>
          <table className="table">
            <thead>
              <tr>
                <th>名称</th>
                <th>描述</th>
                <th>创建时间</th>
                <th style={{ width: 180 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr
                  key={user.id}
                  className={user.id === selectedUserId ? 'table__row--active' : ''}
                >
                  <td>{user.name}</td>
                  <td>{user.description || '-'}</td>
                  <td>{new Date(user.created_at).toLocaleString()}</td>
                  <td>
                    <div className="table-actions">
                      <button
                        className="button button--ghost"
                        onClick={() => setSelectedUserId(user.id)}
                      >
                        查看详情
                      </button>
                      <button
                        className="button button--ghost"
                        onClick={() => handleDeleteUser(user)}
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

      {selectedUser ? (
        <section style={{ display: 'grid', gap: '24px' }}>
          <div className="card">
            <div className="card__header">
              <h2 className="card__title">API Key</h2>
              <div style={{ display: 'flex', gap: '12px' }}>
                <button className="button" onClick={openCreateAPIKeyDialog}>
                  生成 API Key
                </button>
              </div>
            </div>
            {loadingDetails ? (
              <p style={{ padding: '16px 0' }}>加载中...</p>
            ) : apiKeys.length === 0 ? (
              <div className="notice">暂无 API Key，可点击上方按钮创建。</div>
            ) : (
              <div className="table-wrapper">
                <table className="table">
                  <thead>
                    <tr>
                      <th>前缀</th>
                      <th>标签</th>
                      <th>上次使用</th>
                      <th>绑定上游</th>
                      <th style={{ width: 160 }}>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {apiKeys.map((key) => {
                      const binding = bindings[key.id]
                      return (
                        <tr key={key.id}>
                          <td>{key.prefix}</td>
                          <td>{key.label || '-'}</td>
                          <td>
                            {key.last_used_at ? new Date(key.last_used_at).toLocaleString() : '从未使用'}
                          </td>
                          <td>
                            {binding ? (
                              <span>
                                {binding.upstream.provider} · {binding.upstream.label || binding.upstream.id}
                              </span>
                            ) : upstreams.length === 0 ? (
                              <span style={{ color: '#64748b' }}>暂无上游</span>
                            ) : (
                              <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
                                <select
                                  className="field__input field__input--inline"
                                  defaultValue=""
                                  onChange={(event) => {
                                    const upstreamId = event.target.value
                                    if (upstreamId) {
                                      void handleBindAPIKey(key.id, upstreamId)
                                    }
                                  }}
                                >
                                  <option value="">选择上游</option>
                                  {upstreams.map((upstream) => (
                                    <option key={upstream.id} value={upstream.id}>
                                      {upstream.provider} · {upstream.label || upstream.id}
                                    </option>
                                  ))}
                                </select>
                              </div>
                            )}
                          </td>
                          <td>
                            <div className="table-actions">
                              {binding && (
                                <span className="status status--active">已绑定</span>
                              )}
                              <button
                                className="button button--ghost"
                                onClick={() => handleDeleteAPIKey(key)}
                              >
                                撤销
                              </button>
                            </div>
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            )}
            {apiKeySecret ? (
              <div className="alert alert--info" style={{ marginTop: 16 }}>
                <strong>请立即保存：</strong> 新生成的 API Key 为 <code>{apiKeySecret}</code>
              </div>
            ) : null}
          </div>

          <div className="card">
            <div className="card__header">
              <h2 className="card__title">上游凭据</h2>
              <div style={{ display: 'flex', gap: '12px' }}>
                <button className="button" onClick={openCreateUpstreamDialog}>
                  新建凭据
                </button>
              </div>
            </div>
            {loadingDetails ? (
              <p style={{ padding: '16px 0' }}>加载中...</p>
            ) : upstreams.length === 0 ? (
              <div className="notice">暂无上游凭据，点击“新建凭据”以配置访问上游。</div>
            ) : (
              <div className="table-wrapper">
                <table className="table">
                  <thead>
                    <tr>
                      <th>服务商</th>
                      <th>标签</th>
                      <th>可用地址</th>
                      <th style={{ width: 160 }}>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {upstreams.map((upstream) => (
                      <tr key={upstream.id}>
                        <td>{upstream.provider}</td>
                        <td>{upstream.label || '-'}</td>
                        <td>
                          {upstream.endpoints && upstream.endpoints.length > 0 ? (
                            <ul style={{ margin: 0, paddingLeft: 20 }}>
                              {upstream.endpoints.map((endpoint) => (
                                <li key={endpoint}>{endpoint}</li>
                              ))}
                            </ul>
                          ) : (
                            <span>-</span>
                          )}
                        </td>
                        <td>
                          <div className="table-actions">
                            <button
                              className="button button--ghost"
                              onClick={() => handleDeleteUpstream(upstream)}
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
          </div>
        </section>
      ) : null}

      {dialog?.type === 'create-user' ? (
        <div className="dialog-backdrop" role="dialog" aria-modal>
          <form className="dialog" onSubmit={handleCreateUser}>
            <div className="dialog__header">
              <h2 className="dialog__title">新建用户</h2>
              <button type="button" className="dialog__close" onClick={closeDialog}>
                ×
              </button>
            </div>
            <div className="field">
              <span className="field__label">用户名称</span>
              <input
                className="field__input"
                value={userForm.name}
                onChange={(event) => setUserForm((prev) => ({ ...prev, name: event.target.value }))}
                required
              />
            </div>
            <div className="field">
              <span className="field__label">描述</span>
              <textarea
                className="field__textarea"
                rows={3}
                value={userForm.description}
                onChange={(event) =>
                  setUserForm((prev) => ({ ...prev, description: event.target.value }))
                }
              />
            </div>
            <div className="dialog__actions">
              <button type="button" className="button button--ghost" onClick={closeDialog}>
                取消
              </button>
              <button className="button" type="submit">
                保存
              </button>
            </div>
          </form>
        </div>
      ) : null}

      {dialog?.type === 'create-api-key' && selectedUser ? (
        <div className="dialog-backdrop" role="dialog" aria-modal>
          <form className="dialog" onSubmit={handleCreateAPIKey}>
            <div className="dialog__header">
              <h2 className="dialog__title">为 {selectedUser.name} 生成密钥</h2>
              <button type="button" className="dialog__close" onClick={closeDialog}>
                ×
              </button>
            </div>
            <div className="field">
              <span className="field__label">密钥标签</span>
              <input className="field__input" name="label" placeholder="用于区分密钥用途" />
            </div>
            <div className="dialog__actions">
              <button type="button" className="button button--ghost" onClick={closeDialog}>
                取消
              </button>
              <button className="button" type="submit">
                生成
              </button>
            </div>
            {apiKeySecret ? (
              <div className="alert alert--info" style={{ marginTop: 16 }}>
                <strong>请立即保存：</strong> 新生成的 API Key 为 <code>{apiKeySecret}</code>
              </div>
            ) : null}
          </form>
        </div>
      ) : null}

      {dialog?.type === 'create-upstream' && selectedUser ? (
        <div className="dialog-backdrop" role="dialog" aria-modal>
          <form className="dialog" onSubmit={handleCreateUpstream}>
            <div className="dialog__header">
              <h2 className="dialog__title">新建上游凭据</h2>
              <button type="button" className="dialog__close" onClick={closeDialog}>
                ×
              </button>
            </div>
            <div className="field">
              <span className="field__label">服务商</span>
              <input
                className="field__input"
                value={upstreamForm.provider}
                onChange={(event) =>
                  setUpstreamForm((prev) => ({ ...prev, provider: event.target.value }))
                }
                placeholder="如 openai、anthropic"
                required
              />
            </div>
            <div className="field">
              <span className="field__label">标签</span>
              <input
                className="field__input"
                value={upstreamForm.label}
                onChange={(event) =>
                  setUpstreamForm((prev) => ({ ...prev, label: event.target.value }))
                }
                placeholder="用于标识环境或用途"
              />
            </div>
            <div className="field">
              <span className="field__label">上游 API Key</span>
              <input
                className="field__input"
                value={upstreamForm.plaintext}
                onChange={(event) =>
                  setUpstreamForm((prev) => ({ ...prev, plaintext: event.target.value }))
                }
                placeholder="请粘贴上游服务商提供的 API Key"
                required
              />
            </div>
            <div className="field">
              <span className="field__label">可用地址</span>
              <textarea
                className="field__textarea"
                rows={3}
                value={upstreamForm.endpoints}
                onChange={(event) =>
                  setUpstreamForm((prev) => ({ ...prev, endpoints: event.target.value }))
                }
                placeholder="每行一个地址，例如 https://api.openai.com/v1"
              />
            </div>
            <div className="dialog__actions">
              <button type="button" className="button button--ghost" onClick={closeDialog}>
                取消
              </button>
              <button className="button" type="submit">
                保存
              </button>
            </div>
          </form>
        </div>
      ) : null}
    </div>
  )
}

export default UsersPage
