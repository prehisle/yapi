import { useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'

import type { Rule } from '../types/rule'
import { Button } from './ui/Button'

export type RuleFormDialogProps = {
  open: boolean
  mode: 'create' | 'edit'
  initialRule?: Rule
  onClose: () => void
  onSubmit: (payload: Rule) => Promise<void>
}

type FormValues = {
  id: string
  priority: string
  pathPrefix: string
  methods: string
  apiKeyIDs: string
  apiKeyPrefixes: string
  userIDs: string
  userMetadata: string
  bindingUpstreamIDs: string
  bindingProviders: string
  requireBinding: boolean
  targetURL: string
  enabled: boolean
  setHeaders: string
  addHeaders: string
  removeHeaders: string
  setAuthorization: string
  overrideJSON: string
  removeJSON: string
}

type FormErrors = Partial<Record<keyof FormValues, string>>

const defaultValues: FormValues = {
  id: '',
  priority: '',
  pathPrefix: '',
  methods: '',
  apiKeyIDs: '',
  apiKeyPrefixes: '',
  userIDs: '',
  userMetadata: '',
  bindingUpstreamIDs: '',
  bindingProviders: '',
  requireBinding: false,
  targetURL: '',
  enabled: true,
  setHeaders: '',
  addHeaders: '',
  removeHeaders: '',
  setAuthorization: '',
  overrideJSON: '',
  removeJSON: '',
}

const stringifyRecord = (record?: Record<string, string>) => {
  if (!record) return ''
  return Object.entries(record)
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

const stringifyList = (items?: string[]) => {
  if (!items || items.length === 0) return ''
  return items.join('\n')
}

const stringifyJSON = (record?: Record<string, unknown>) => {
  if (!record) return ''
  try {
    return JSON.stringify(record, null, 2)
  } catch {
    return ''
  }
}

const parseKeyValueBlock = (input: string, fieldName: string) => {
  if (!input.trim()) return undefined
  const result: Record<string, string> = {}
  const lines = input.split(/\n+/)
  for (const raw of lines) {
    const line = raw.trim()
    if (!line) continue
    const [key, value] = line.split('=')
    if (!key || value === undefined) {
      throw new Error(`${fieldName} 格式需为 key=value，每行一条`)
    }
    result[key.trim()] = value.trim()
  }
  return Object.keys(result).length > 0 ? result : undefined
}

const parseList = (input: string) => {
  if (!input.trim()) return undefined
  const items = input
    .split(/[\n,]/)
    .map((item) => item.trim())
    .filter(Boolean)
  return items.length > 0 ? items : undefined
}

const parseOverrideJSON = (input: string) => {
  if (!input.trim()) return undefined
  try {
    const value = JSON.parse(input)
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      throw new Error()
    }
    return value as Record<string, unknown>
  } catch (err) {
    throw new Error(`override_json 解析失败: ${(err as Error).message}`)
  }
}

const ruleToValues = (rule: Rule): FormValues => ({
  id: rule.id,
  priority: String(rule.priority),
  pathPrefix: rule.matcher.path_prefix ?? '',
  methods: rule.matcher.methods?.join(', ') ?? '',
  apiKeyIDs: stringifyList(rule.matcher.api_key_ids),
  apiKeyPrefixes: stringifyList(rule.matcher.api_key_prefixes),
  userIDs: stringifyList(rule.matcher.user_ids),
  userMetadata: stringifyRecord(rule.matcher.user_metadata),
  bindingUpstreamIDs: stringifyList(rule.matcher.binding_upstream_ids),
  bindingProviders: stringifyList(rule.matcher.binding_providers),
  requireBinding: rule.matcher.require_binding ?? false,
  targetURL: rule.actions.set_target_url ?? '',
  enabled: rule.enabled,
  setHeaders: stringifyRecord(rule.actions.set_headers),
  addHeaders: stringifyRecord(rule.actions.add_headers),
  removeHeaders: rule.actions.remove_headers?.join(', ') ?? '',
  setAuthorization: rule.actions.set_authorization ?? '',
  overrideJSON: stringifyJSON(rule.actions.override_json),
  removeJSON: rule.actions.remove_json?.join(', ') ?? '',
})

const validate = (values: FormValues): FormErrors => {
  const errors: FormErrors = {}
  if (!values.id.trim()) {
    errors.id = '规则 ID 必填'
  } else if (!/^[A-Za-z0-9_-]+$/.test(values.id.trim())) {
    errors.id = '仅允许字母、数字、下划线与中划线'
  }

  if (!values.priority.trim()) {
    errors.priority = '优先级必填'
  } else if (!/^(-?\d+)$/.test(values.priority.trim())) {
    errors.priority = '优先级必须为整数'
  }

  if (values.apiKeyPrefixes.trim()) {
    const prefixes = parseList(values.apiKeyPrefixes) ?? []
    const invalid = prefixes.find((prefix) => !/^[A-Za-z0-9]{8}$/.test(prefix))
    if (invalid) {
      errors.apiKeyPrefixes = 'API Key 前缀需为 8 位字母或数字'
    }
  }

  if (values.targetURL.trim()) {
    try {
      new URL(values.targetURL.trim())
    } catch {
      errors.targetURL = '目标地址必须是合法 URL'
    }
  }

  return errors
}

const buildPayload = (values: FormValues): Rule => {
  const setHeaders = parseKeyValueBlock(values.setHeaders, 'set_headers')
  const addHeaders = parseKeyValueBlock(values.addHeaders, 'add_headers')
  const removeHeaders = parseList(values.removeHeaders)
  const overrideJSON = parseOverrideJSON(values.overrideJSON)
  const removeJSON = parseList(values.removeJSON)

  const methods = parseList(values.methods)?.map((method) => method.toUpperCase())
  const apiKeyIDs = parseList(values.apiKeyIDs)
  const apiKeyPrefixes = parseList(values.apiKeyPrefixes)
  const userIDs = parseList(values.userIDs)
  const userMetadata = parseKeyValueBlock(values.userMetadata, 'user_metadata')
  const bindingUpstreamIDs = parseList(values.bindingUpstreamIDs)
  const bindingProviders = parseList(values.bindingProviders)

  return {
    id: values.id.trim(),
    priority: Number(values.priority.trim()),
    enabled: values.enabled,
    matcher: {
      path_prefix: values.pathPrefix.trim() || undefined,
      methods: methods,
      api_key_ids: apiKeyIDs,
      api_key_prefixes: apiKeyPrefixes,
      user_ids: userIDs,
      user_metadata: userMetadata,
      binding_upstream_ids: bindingUpstreamIDs,
      binding_providers: bindingProviders,
      require_binding: values.requireBinding || undefined,
    },
    actions: {
      set_target_url: values.targetURL.trim() || undefined,
      set_headers: setHeaders,
      add_headers: addHeaders,
      remove_headers: removeHeaders,
      set_authorization: values.setAuthorization.trim() || undefined,
      override_json: overrideJSON,
      remove_json: removeJSON,
    },
  }
}

export const RuleFormDialog = ({
  open,
  mode,
  initialRule,
  onClose,
  onSubmit,
}: RuleFormDialogProps) => {
  const [values, setValues] = useState<FormValues>(defaultValues)
  const [errors, setErrors] = useState<FormErrors>({})
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) return
    if (mode === 'edit' && initialRule) {
      setValues(ruleToValues(initialRule))
    } else {
      setValues(defaultValues)
    }
    setErrors({})
    setSubmitError(null)
    setSubmitting(false)
  }, [open, mode, initialRule])

  const dialogTitle = useMemo(() => (mode === 'edit' ? '编辑规则' : '新建规则'), [mode])

  const handleChange = (key: keyof FormValues) => (event: FormEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    const target = event.target as HTMLInputElement | HTMLTextAreaElement
    const next: FormValues = {
      ...values,
      [key]: target.type === 'checkbox' ? (target as HTMLInputElement).checked : target.value,
    }
    setValues(next)
    setErrors((prev) => ({ ...prev, ...validate(next) }))
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setSubmitError(null)

    const validation = validate(values)
    setErrors(validation)
    if (Object.keys(validation).length > 0) {
      return
    }

    try {
      const payload = buildPayload(values)
      setSubmitting(true)
      await onSubmit(payload)
    } catch (err) {
      setSubmitError((err as Error).message || '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  if (!open) return null

  return (
    <div className="dialog-backdrop" role="dialog" aria-modal>
      <form className="dialog dialog--full" onSubmit={handleSubmit}>
        <div className="dialog__header">
          <h2 className="dialog__title">{dialogTitle}</h2>
          <Button type="button" variant="ghost" size="sm" onClick={onClose} aria-label="关闭">
            ×
          </Button>
        </div>

        <section className="form-section">
          <h3 className="form-section__title">基础信息</h3>
          <div className="field">
            <span className="field__label">规则 ID</span>
            <input
              className="field__input"
              value={values.id}
              onChange={handleChange('id')}
              disabled={mode === 'edit'}
              required
            />
            {errors.id ? <p className="field__error">{errors.id}</p> : null}
          </div>

          <div className="field">
            <span className="field__label">优先级</span>
            <input
              className="field__input"
              type="number"
              value={values.priority}
              onChange={handleChange('priority')}
              required
            />
            {errors.priority ? <p className="field__error">{errors.priority}</p> : null}
          </div>

          <div className="field">
            <span className="field__label">路径前缀</span>
            <input
              className="field__input"
              value={values.pathPrefix}
              onChange={handleChange('pathPrefix')}
              placeholder="例如 /v1/chat"
            />
          </div>

          <div className="field">
            <span className="field__label">方法列表</span>
            <input
              className="field__input"
              value={values.methods}
              onChange={handleChange('methods')}
              placeholder="逗号分隔，例如 POST,GET"
            />
            <span className="field__hint">留空表示匹配所有方法。</span>
          </div>

          <div className="field">
            <span className="field__label">目标地址</span>
            <input
              className="field__input"
              value={values.targetURL}
              onChange={handleChange('targetURL')}
              placeholder="https://api.openai.com"
            />
            {errors.targetURL ? <p className="field__error">{errors.targetURL}</p> : null}
          </div>

          <label className="field field--inline">
            <input
              type="checkbox"
              checked={values.enabled}
              onChange={handleChange('enabled')}
            />
            <span className="field__label">启用规则</span>
          </label>
        </section>

        <section className="form-section">
          <h3 className="form-section__title">账户上下文匹配</h3>
          <div className="field">
            <span className="field__label">API Key ID</span>
            <textarea
              className="field__textarea"
              placeholder="逐行填写完整的 API Key ID"
              value={values.apiKeyIDs}
              onChange={handleChange('apiKeyIDs')}
              rows={2}
            />
          </div>

          <div className="field">
            <span className="field__label">API Key 前缀</span>
            <textarea
              className="field__textarea"
              placeholder="逐行填写 8 位前缀，例如 abcd1234"
              value={values.apiKeyPrefixes}
              onChange={handleChange('apiKeyPrefixes')}
              rows={2}
            />
            {errors.apiKeyPrefixes ? <p className="field__error">{errors.apiKeyPrefixes}</p> : null}
          </div>

          <div className="field">
            <span className="field__label">用户 ID</span>
            <textarea
              className="field__textarea"
              placeholder="逐行填写用户 ID"
              value={values.userIDs}
              onChange={handleChange('userIDs')}
              rows={2}
            />
          </div>

          <div className="field">
            <span className="field__label">用户元数据匹配</span>
            <textarea
              className="field__textarea"
              placeholder={`key=value\nregion=eu-west-1`}
              value={values.userMetadata}
              onChange={handleChange('userMetadata')}
              rows={3}
            />
            <span className="field__hint">需要与用户 Metadata 完全匹配对应键值。</span>
          </div>

          <div className="field">
            <span className="field__label">上游凭据 ID</span>
            <textarea
              className="field__textarea"
              placeholder="逐行填写上游凭据 ID"
              value={values.bindingUpstreamIDs}
              onChange={handleChange('bindingUpstreamIDs')}
              rows={2}
            />
          </div>

          <div className="field">
            <span className="field__label">上游 Provider</span>
            <textarea
              className="field__textarea"
              placeholder="逐行填写 Provider 标识，例如 openai"
              value={values.bindingProviders}
              onChange={handleChange('bindingProviders')}
              rows={2}
            />
          </div>

          <label className="field field--inline">
            <input
              type="checkbox"
              checked={values.requireBinding}
              onChange={handleChange('requireBinding')}
            />
            <span className="field__label">仅在存在上游绑定时匹配</span>
          </label>
          <span className="field__hint">
            启用后，若请求未解析出 API Key 绑定信息，则不会命中该规则。
          </span>
        </section>

        <section className="form-section">
          <h3 className="form-section__title">头部修改</h3>
          <div className="field">
            <span className="field__label">覆盖头部（set_headers）</span>
            <textarea
              className="field__textarea"
              placeholder={`key=value\nAuthorization=Bearer xxx`}
              value={values.setHeaders}
              onChange={handleChange('setHeaders')}
              rows={3}
            />
            <span className="field__hint">逐行填写 key=value，将覆盖现有头部。</span>
          </div>

          <div className="field">
            <span className="field__label">追加头部（add_headers）</span>
            <textarea
              className="field__textarea"
              placeholder={`X-Debug=true`}
              value={values.addHeaders}
              onChange={handleChange('addHeaders')}
              rows={3}
            />
            <span className="field__hint">逐行填写 key=value，将在原有头部基础上追加。</span>
          </div>

          <div className="field">
            <span className="field__label">移除头部（remove_headers）</span>
            <textarea
              className="field__textarea"
              placeholder="header1, header2"
              value={values.removeHeaders}
              onChange={handleChange('removeHeaders')}
              rows={2}
            />
            <span className="field__hint">可用逗号或换行分隔多个头部。</span>
          </div>

          <div className="field">
            <span className="field__label">Authorization 头</span>
            <input
              className="field__input"
              value={values.setAuthorization}
              onChange={handleChange('setAuthorization')}
              placeholder="Bearer xxx"
            />
          </div>
        </section>

        <section className="form-section">
          <h3 className="form-section__title">请求体动作</h3>
          <div className="field">
            <span className="field__label">override_json</span>
            <textarea
              className="field__textarea"
              placeholder={`{\n  "model": "gpt-4.1"\n}`}
              value={values.overrideJSON}
              onChange={handleChange('overrideJSON')}
              rows={4}
            />
            <span className="field__hint">需填写 JSON 对象，留空表示不修改。</span>
          </div>

          <div className="field">
            <span className="field__label">remove_json</span>
            <textarea
              className="field__textarea"
              placeholder="messages[0].content, metadata.trace_id"
              value={values.removeJSON}
              onChange={handleChange('removeJSON')}
              rows={2}
            />
            <span className="field__hint">支持点号与数组下标语法，留空表示不移除。</span>
          </div>
        </section>

        {submitError ? <div className="alert alert--error">{submitError}</div> : null}

        <div className="dialog__actions">
          <Button
            type="button"
            variant="ghost"
            onClick={onClose}
            disabled={submitting}
          >
            取消
          </Button>
          <Button type="submit" loading={submitting}>
            {submitting ? '提交中...' : mode === 'edit' ? '保存' : '创建'}
          </Button>
        </div>
      </form>
    </div>
  )
}
