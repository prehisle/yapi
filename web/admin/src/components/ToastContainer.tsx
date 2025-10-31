import { useUIContext } from '../context/UIContext'

const ToastContainer = () => {
  const { toasts, removeToast } = useUIContext()

  if (toasts.length === 0) return null

  return (
    <div className="toast-container" role="status" aria-live="polite">
      {toasts.map((toast) => (
        <div key={toast.id} className={`toast toast--${toast.type}`}>
          <span>{toast.message}</span>
          <button
            type="button"
            className="toast__close"
            aria-label="关闭提示"
            onClick={() => removeToast(toast.id)}
          >
            ×
          </button>
        </div>
      ))}
    </div>
  )
}

export default ToastContainer
