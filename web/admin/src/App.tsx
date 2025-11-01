import { Navigate, Outlet, Route, Routes } from 'react-router-dom'

import { useAuth } from './hooks/useAuth'
import ToastContainer from './components/ToastContainer'
import ConfirmDialog from './components/ConfirmDialog'
import AdminLayout from './components/AdminLayout'
import LoginPage from './pages/LoginPage'
import RulesPage from './pages/RulesPage'
import UsersPage from './pages/UsersPage'

const RequireAuth = () => {
  const { token, loading } = useAuth()
  if (loading) {
    return <div className="page page--center">正在加载...</div>
  }
  if (!token) {
    return <Navigate to="/login" replace />
  }
  return <Outlet />
}

function App() {
  return (
    <>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route element={<RequireAuth />}>
          <Route element={<AdminLayout />}>
            <Route path="/" element={<RulesPage />} />
            <Route path="/users" element={<UsersPage />} />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ToastContainer />
      <ConfirmDialog />
    </>
  )
}

export default App
