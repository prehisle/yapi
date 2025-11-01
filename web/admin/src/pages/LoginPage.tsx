import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuth } from '../hooks/useAuth'
import { Input } from '../components/ui/Input'
import { Button } from '../components/ui/Button'
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/Card'

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
      <Card className="login-card max-w-md w-full">
        <CardHeader>
          <CardTitle className="text-center">管理员登录</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-6">
            <Input
              id="username"
              label="用户名"
              type="text"
              autoComplete="username"
              value={form.username}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, username: event.target.value }))
              }
              required
              placeholder="请输入管理员用户名"
              autoFocus
            />

            <Input
              id="password"
              label="密码"
              type="password"
              autoComplete="current-password"
              value={form.password}
              onChange={(event) =>
                setForm((prev) => ({ ...prev, password: event.target.value }))
              }
              required
              placeholder="请输入密码"
              error={error || undefined}
            />

            <Button
              type="submit"
              loading={loading}
              fullWidth
              size="lg"
            >
              {loading ? '登录中...' : '登录'}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

export default LoginPage
