import type { Rule } from '../types/rule'
import { RuleDiffViewer } from './RuleDiffViewer'

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

  return (
    <div className="drawer-backdrop" role="dialog" aria-modal>
      <aside className="drawer">
        <header className="drawer__header">
          <div>
            <h2 className="drawer__title">规则详情</h2>
            <p className="drawer__subtitle">{rule.id}</p>
          </div>
          <button className="drawer__close" onClick={onClose} aria-label="关闭">
            ×
          </button>
        </header>

        <section className="drawer-section">
          <h3 className="drawer-section__title">基础信息</h3>
          <dl className="description-list">
            <div>
              <dt>ID</dt>
              <dd>{rule.id}</dd>
            </div>
            <div>
              <dt>优先级</dt>
              <dd>{rule.priority}</dd>
            </div>
            <div>
              <dt>路径前缀</dt>
              <dd>{rule.matcher.path_prefix ?? '-'}</dd>
            </div>
            <div>
              <dt>方法</dt>
              <dd>{methods}</dd>
            </div>
            <div>
              <dt>目标地址</dt>
              <dd>{rule.actions.set_target_url ?? '-'}</dd>
            </div>
            <div>
              <dt>状态</dt>
              <dd>{rule.enabled ? '已启用' : '未启用'}</dd>
            </div>
          </dl>
        </section>

        <section className="drawer-section">
          <h3 className="drawer-section__title">头部动作</h3>
          <div className="drawer-table">
            <h4>覆盖头部（set_headers）</h4>
            {setHeaders.length > 0 ? (
              <table>
                <tbody>
                  {setHeaders.map((item) => (
                    <tr key={item.key}>
                      <td>{item.key}</td>
                      <td>{item.value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <p className="drawer-empty">-</p>
            )}
          </div>

          <div className="drawer-table">
            <h4>追加头部（add_headers）</h4>
            {addHeaders.length > 0 ? (
              <table>
                <tbody>
                  {addHeaders.map((item) => (
                    <tr key={item.key}>
                      <td>{item.key}</td>
                      <td>{item.value}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : (
              <p className="drawer-empty">-</p>
            )}
          </div>

          <div className="drawer-table">
            <h4>移除头部（remove_headers）</h4>
            {removeHeaders.length > 0 ? (
              <ul>
                {removeHeaders.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            ) : (
              <p className="drawer-empty">-</p>
            )}
          </div>

          <dl className="description-list">
            <div>
              <dt>Authorization 头</dt>
              <dd>{rule.actions.set_authorization ?? '-'}</dd>
            </div>
          </dl>
        </section>

        <section className="drawer-section">
          <h3 className="drawer-section__title">请求体动作</h3>
          <div className="drawer-code">
            <h4>override_json</h4>
            <pre>{stringifyJSON(rule.actions.override_json)}</pre>
          </div>
          <div className="drawer-table">
            <h4>remove_json</h4>
            {rule.actions.remove_json?.length ? (
              <ul>
                {rule.actions.remove_json.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            ) : (
              <p className="drawer-empty">-</p>
            )}
          </div>
        </section>

        <section className="drawer-section">
          <h3 className="drawer-section__title">变更对比</h3>
          <RuleDiffViewer previous={previousRule} current={rule} />
        </section>

        <footer className="drawer__footer">
          <button className="button button--ghost" onClick={onClose}>
            关闭
          </button>
        </footer>
      </aside>
    </div>
  )
}
