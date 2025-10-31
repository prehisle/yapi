import { useMemo } from 'react'

import type { Rule } from '../types/rule'

type RuleDiffViewerProps = {
  previous?: Rule
  current: Rule
}

type ChangeItem = {
  field: string
  before: string
  after: string
}

const stringify = (value: unknown) => {
  if (value === undefined || value === null) return '-'
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

const collectChanges = (prev: Rule | undefined, curr: Rule): ChangeItem[] => {
  const changes: ChangeItem[] = []
  const compare = (field: string, before: unknown, after: unknown) => {
    if (JSON.stringify(before) !== JSON.stringify(after)) {
      changes.push({
        field,
        before: stringify(before),
        after: stringify(after),
      })
    }
  }

  if (!prev) {
    // 如果没有 previous，将所有字段视为新增
    compare('priority', undefined, curr.priority)
    compare('matcher.path_prefix', undefined, curr.matcher.path_prefix)
    compare('matcher.methods', undefined, curr.matcher.methods)
    compare('actions.set_target_url', undefined, curr.actions.set_target_url)
    compare('actions.set_headers', undefined, curr.actions.set_headers)
    compare('actions.add_headers', undefined, curr.actions.add_headers)
    compare('actions.remove_headers', undefined, curr.actions.remove_headers)
    compare('actions.set_authorization', undefined, curr.actions.set_authorization)
    compare('actions.override_json', undefined, curr.actions.override_json)
    compare('actions.remove_json', undefined, curr.actions.remove_json)
    compare('enabled', undefined, curr.enabled)
    return changes
  }

  compare('priority', prev.priority, curr.priority)
  compare('matcher.path_prefix', prev.matcher.path_prefix, curr.matcher.path_prefix)
  compare('matcher.methods', prev.matcher.methods, curr.matcher.methods)
  compare('actions.set_target_url', prev.actions.set_target_url, curr.actions.set_target_url)
  compare('actions.set_headers', prev.actions.set_headers, curr.actions.set_headers)
  compare('actions.add_headers', prev.actions.add_headers, curr.actions.add_headers)
  compare('actions.remove_headers', prev.actions.remove_headers, curr.actions.remove_headers)
  compare('actions.set_authorization', prev.actions.set_authorization, curr.actions.set_authorization)
  compare('actions.override_json', prev.actions.override_json, curr.actions.override_json)
  compare('actions.remove_json', prev.actions.remove_json, curr.actions.remove_json)
  compare('enabled', prev.enabled, curr.enabled)

  return changes
}

export const RuleDiffViewer = ({ previous, current }: RuleDiffViewerProps) => {
  const changes = useMemo(() => collectChanges(previous, current), [previous, current])

  if (changes.length === 0) {
    return <p className="drawer-empty">当前配置与上次无差异</p>
  }

  return (
    <table className="drawer-diff">
      <thead>
        <tr>
          <th>字段</th>
          <th>原值</th>
          <th>现值</th>
        </tr>
      </thead>
      <tbody>
        {changes.map((item) => (
          <tr key={item.field}>
            <td>{item.field}</td>
            <td>
              <pre>{item.before}</pre>
            </td>
            <td>
              <pre>{item.after}</pre>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
