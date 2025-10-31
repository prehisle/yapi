import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'
import type { PropsWithChildren } from 'react'

import { apiClient } from '../lib/api'

type AuthState = {
  token: string | null
  loading: boolean
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState | undefined>(undefined)

const STORAGE_KEY = 'yapi_admin_token'

export const AuthProvider = ({ children }: PropsWithChildren) => {
  const [token, setToken] = useState<string | null>(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      apiClient.setToken(stored)
    }
    return stored
  })
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    apiClient.setToken(token)
  }, [token])

  const login = useCallback(async (username: string, password: string) => {
    setLoading(true)
    try {
      const result = await apiClient.post<{ access_token: string }>(
        '/admin/login',
        {
          username,
          password,
        },
      )
      apiClient.setToken(result.access_token)
      setToken(result.access_token)
      localStorage.setItem(STORAGE_KEY, result.access_token)
    } finally {
      setLoading(false)
    }
  }, [])

  const logout = useCallback(() => {
    apiClient.setToken(null)
    setToken(null)
    localStorage.removeItem(STORAGE_KEY)
  }, [])

  const value = useMemo(
    () => ({
      token,
      loading,
      login,
      logout,
    }),
    [token, loading, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export const useAuthContext = () => {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuthContext must be used within AuthProvider')
  }
  return context
}
