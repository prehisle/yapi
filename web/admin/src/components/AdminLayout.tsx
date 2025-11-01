import { NavLink, Outlet } from 'react-router-dom'

import { useAuth } from '../hooks/useAuth'

const linkClass = ({ isActive }: { isActive: boolean }) =>
  `app-nav__link${isActive ? ' app-nav__link--active' : ''}`

const AdminLayout = () => {
  const { logout } = useAuth()

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-header__inner">
          <div>
            <h1 className="app-header__title">YAPI 管理后台</h1>
            <p className="app-header__subtitle">统一配置规则、用户与上游凭据</p>
          </div>
          <nav className="app-nav" aria-label="主导航">
            <NavLink to="/" end className={linkClass}>
              规则管理
            </NavLink>
            <NavLink to="/users" className={linkClass}>
              用户管理
            </NavLink>
          </nav>
          <button className="button button--ghost" onClick={logout}>
            退出登录
          </button>
        </div>
      </header>
      <main className="app-main">
        <Outlet />
      </main>
    </div>
  )
}

export default AdminLayout
