import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuth } from '../hooks/useAuth'

const LoginPage = () => {
  const { login, loading, token } = useAuth()
  const navigate = useNavigate()
  const [form, setForm] = useState({ username: '', password: '' })
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (token) {
      navigate('/', { replace: true })
    }
  }, [token, navigate])

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError(null)
    try {
      await login(form.username, form.password)
      navigate('/', { replace: true })
    } catch (err) {
      setError((err as Error).message || '登录失败')
    }
  }

  return (
    <div className="page page--center">
      <form className="card login-card" onSubmit={handleSubmit}>
        <h1 className="card__title">管理员登录</h1>
        <label className="field">
          <span className="field__label">用户名</span>
          <input
            className="field__input"
            type="text"
            autoComplete="username"
            value={form.username}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, username: event.target.value }))
            }
            required
          />
        </label>
        <label className="field">
          <span className="field__label">密码</span>
          <input
            className="field__input"
            type="password"
            autoComplete="current-password"
            value={form.password}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, password: event.target.value }))
            }
            required
          />
        </label>
        {error ? <p className="field__error">{error}</p> : null}
        <button className="button" type="submit" disabled={loading}>
          {loading ? '登录中...' : '登录'}
        </button>
      </form>
    </div>
  )
}

export default LoginPage
