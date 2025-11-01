export interface RuleMatcher {
  path_prefix?: string
  methods?: string[]
  headers?: Record<string, string>
  api_key_ids?: string[]
  api_key_prefixes?: string[]
  user_ids?: string[]
  user_metadata?: Record<string, string>
  binding_upstream_ids?: string[]
  binding_providers?: string[]
  require_binding?: boolean
}

export interface RewritePathExpression {
  pattern: string
  replace: string
}

export interface RuleActions {
  set_target_url?: string
  set_headers?: Record<string, string>
  add_headers?: Record<string, string>
  remove_headers?: string[]
  set_authorization?: string
  override_json?: Record<string, unknown>
  remove_json?: string[]
  rewrite_path_regex?: RewritePathExpression
  script?: string
}

export interface Rule {
  id: string
  priority: number
  matcher: RuleMatcher
  actions: RuleActions
  enabled: boolean
}

export interface RuleListResponse {
  items: Rule[]
  total: number
  enabled_total: number
  page: number
  page_size: number
}
