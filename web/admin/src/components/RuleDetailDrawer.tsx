import type { Rule } from '../types/rule'
import { RuleDiffViewer } from './RuleDiffViewer'
import { Button } from './ui/Button'
import { InfoCard, InfoItem } from './ui/InfoCard'

export type RuleDetailDrawerProps = {
  open: boolean
  rule?: Rule
  previousRule?: Rule
  onClose: () => void
}

type KeyValue = {
  key: string
  value: string
}

const partitionHeaders = (record?: Record<string, string>): KeyValue[] => {
  if (!record) return []
  return Object.entries(record).map(([key, value]) => ({ key, value }))
}

const stringifyJSON = (value?: Record<string, unknown>) => {
  if (!value) return '-'
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return '-'
  }
}

export const RuleDetailDrawer = ({ open, rule, previousRule, onClose }: RuleDetailDrawerProps) => {
  if (!open || !rule) return null

  const setHeaders = partitionHeaders(rule.actions.set_headers)
  const addHeaders = partitionHeaders(rule.actions.add_headers)
  const removeHeaders = rule.actions.remove_headers ?? []
  const methods = rule.matcher.methods?.join(', ') ?? '所有方法'
  const apiKeyIDs = rule.matcher.api_key_ids ?? []
  const apiKeyPrefixes = rule.matcher.api_key_prefixes ?? []
  const userIDs = rule.matcher.user_ids ?? []
  const userMetadata = partitionHeaders(rule.matcher.user_metadata)
  const upstreamIDs = rule.matcher.binding_upstream_ids ?? []
  const upstreamProviders = rule.matcher.binding_providers ?? []

  return (
    <div className="drawer-backdrop" role="dialog" aria-modal>
      <aside className="drawer">
        <header className="drawer__header">
          <div>
            <h2 className="drawer__title flex items-center gap-2">
              规则详情
              <span className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${
                rule.enabled
                  ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                  : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
              }`}>
                {rule.enabled ? '已启用' : '未启用'}
              </span>
            </h2>
            <p className="drawer__subtitle">ID: {rule.id} | 优先级: {rule.priority}</p>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose} aria-label="关闭">
            ×
          </Button>
        </header>

        <div className="drawer__content">
          <div className="space-y-6">
            <InfoCard
              title="基础信息"
              icon={
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M12 2L2 7l10 5 10-5-10-5z" />
                  <path d="M2 17l10 5 10-5M2 12l10 5 10-5" />
                </svg>
              }
            >
              <div className="space-y-0">
                <InfoItem label="ID" value={rule.id} copyable />
                <InfoItem label="优先级" value={rule.priority} />
                <InfoItem label="路径前缀" value={rule.matcher.path_prefix || '-'} copyable />
                <InfoItem label="方法" value={methods} />
                <InfoItem label="目标地址" value={rule.actions.set_target_url || '-'} copyable />
              </div>
            </InfoCard>

            <InfoCard
              title="账户上下文匹配"
              icon={
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                  <circle cx="12" cy="7" r="4" />
                </svg>
              }
              badge={
                <span className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${
                  rule.matcher.require_binding
                    ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                    : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
                }`}>
                  {rule.matcher.require_binding ? '需要绑定' : '可选绑定'}
                </span>
              }
            >
              <div className="space-y-3">
                {apiKeyIDs.length > 0 && (
                  <div>
                    <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">API Key ID</h5>
                    <div className="flex flex-wrap gap-1">
                      {apiKeyIDs.map((id) => (
                        <span key={id} className="inline-flex items-center px-2 py-1 rounded-md text-xs font-mono bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
                          {id}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {apiKeyPrefixes.length > 0 && (
                  <div>
                    <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">API Key 前缀</h5>
                    <div className="flex flex-wrap gap-1">
                      {apiKeyPrefixes.map((prefix) => (
                        <span key={prefix} className="inline-flex items-center px-2 py-1 rounded-md text-xs font-mono bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
                          {prefix}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {userIDs.length > 0 && (
                  <div>
                    <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">用户 ID</h5>
                    <div className="flex flex-wrap gap-1">
                      {userIDs.map((id) => (
                        <span key={id} className="inline-flex items-center px-2 py-1 rounded-md text-xs font-mono bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
                          {id}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {userMetadata.length > 0 && (
                  <div>
                    <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">用户 Metadata</h5>
                    <div className="space-y-1">
                      {userMetadata.map(({ key, value }) => (
                        <div key={key} className="flex items-center gap-2 text-xs">
                          <span className="font-mono bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-300 px-1 py-0.5 rounded">
                            {key}
                          </span>
                          <span className="text-gray-600 dark:text-gray-400">:</span>
                          <span className="font-mono text-gray-800 dark:text-gray-200">{value}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {upstreamIDs.length > 0 && (
                  <div>
                    <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">绑定上游 ID</h5>
                    <div className="flex flex-wrap gap-1">
                      {upstreamIDs.map((id) => (
                        <span key={id} className="inline-flex items-center px-2 py-1 rounded-md text-xs font-mono bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
                          {id}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {upstreamProviders.length > 0 && (
                  <div>
                    <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">绑定上游提供商</h5>
                    <div className="flex flex-wrap gap-1">
                      {upstreamProviders.map((provider) => (
                        <span key={provider} className="inline-flex items-center px-2 py-1 rounded-md text-xs bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
                          {provider}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {!apiKeyIDs.length && !apiKeyPrefixes.length && !userIDs.length && !userMetadata.length && !upstreamIDs.length && !upstreamProviders.length && (
                  <p className="text-sm text-gray-500 dark:text-gray-400 italic">无账户上下文匹配条件</p>
                )}
              </div>
            </InfoCard>

            <InfoCard
              title="头部动作"
              icon={
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <path d="M4 9V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v4" />
                  <path d="M22 15v4a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-4" />
                </svg>
              }
            >
              {setHeaders.length > 0 && (
                <div>
                  <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">覆盖头部（set_headers）</h5>
                  <div className="space-y-1">
                    {setHeaders.map(({ key, value }) => (
                      <div key={key} className="flex items-center gap-2 text-xs">
                        <span className="font-mono bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300 px-1 py-0.5 rounded">
                          {key}
                        </span>
                        <span className="text-gray-600 dark:text-gray-400">:</span>
                        <span className="font-mono text-gray-800 dark:text-gray-200">{value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {addHeaders.length > 0 && (
                <div>
                  <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">添加头部（add_headers）</h5>
                  <div className="space-y-1">
                    {addHeaders.map(({ key, value }) => (
                      <div key={key} className="flex items-center gap-2 text-xs">
                        <span className="font-mono bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 px-1 py-0.5 rounded">
                          {key}
                        </span>
                        <span className="text-gray-600 dark:text-gray-400">:</span>
                        <span className="font-mono text-gray-800 dark:text-gray-200">{value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {removeHeaders.length > 0 && (
                <div>
                  <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">移除头部（remove_headers）</h5>
                  <div className="flex flex-wrap gap-1">
                    {removeHeaders.map((header) => (
                      <span key={header} className="inline-flex items-center px-2 py-1 rounded-md text-xs font-mono bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200 line-through">
                        {header}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {rule.actions.set_authorization && (
                <div>
                  <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">授权头部</h5>
                  <div className="font-mono text-xs text-gray-800 dark:text-gray-200 bg-gray-100 dark:bg-gray-700 p-2 rounded">
                    {rule.actions.set_authorization}
                  </div>
                </div>
              )}

              {!setHeaders.length && !addHeaders.length && !removeHeaders.length && !rule.actions.set_authorization && (
                <p className="text-sm text-gray-500 dark:text-gray-400 italic">无头部动作</p>
              )}
            </InfoCard>

            {rule.actions.override_json && (
              <InfoCard
                title="请求体动作"
                icon={
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M14.5 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7.5L14.5 2z" />
                    <polyline points="14,2 14,8 20,8" />
                  </svg>
                }
              >
                <div>
                  <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">override_json</h5>
                  <div className="bg-gray-900 text-green-400 p-3 rounded-md overflow-x-auto">
                    <pre className="text-xs font-mono">
                      {stringifyJSON(rule.actions.override_json)}
                    </pre>
                  </div>
                </div>

                {rule.actions.remove_json && rule.actions.remove_json.length > 0 && (
                  <div className="mt-4">
                    <h5 className="text-xs font-semibold text-gray-700 dark:text-gray-300 mb-2">remove_json</h5>
                    <div className="flex flex-wrap gap-1">
                      {rule.actions.remove_json.map((path) => (
                        <span key={path} className="inline-flex items-center px-2 py-1 rounded-md text-xs font-mono bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200">
                          {path}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </InfoCard>
            )}

            {previousRule && (
              <InfoCard
                title="变更对比"
                icon={
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                    <path d="M9 11H3v6h6v-6z" />
                    <path d="M21 11h-6v6h6v-6z" />
                    <path d="M5 19v2h2" />
                    <path d="M17 19v2h2" />
                    <path d="M5 3V1H3" />
                    <path d="M17 3V1h2" />
                  </svg>
                }
              >
                <RuleDiffViewer previous={previousRule} current={rule} />
              </InfoCard>
            )}
          </div>
        </div>

        <footer className="drawer__footer">
          <Button variant="ghost" onClick={onClose}>
            关闭
          </Button>
        </footer>
      </aside>
    </div>
  )
}
