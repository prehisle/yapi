export type User = {
  id: string
  name: string
  description?: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type UserListResponse = {
  items: User[]
}

export type APIKey = {
  id: string
  user_id: string
  label?: string
  prefix: string
  last_used_at?: string | null
  created_at: string
  updated_at: string
}

export type APIKeyListResponse = {
  items: APIKey[]
}

export type APIKeyCreateResponse = {
  api_key: APIKey
  secret: string
}

export type UpstreamCredential = {
  id: string
  user_id: string
  provider: string
  label?: string
  endpoints?: string[]
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export type UpstreamCredentialListResponse = {
  items: UpstreamCredential[]
}

export type APIKeyBinding = {
  id: string
  user_id: string
  user_api_key_id: string
  upstream_credential_id: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
  upstream: UpstreamCredential
}
