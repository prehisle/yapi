export interface RuleMatcher {
  path_prefix?: string
  methods?: string[]
  headers?: Record<string, string>
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
