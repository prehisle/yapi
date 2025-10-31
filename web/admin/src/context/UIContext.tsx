import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from 'react'
import type { PropsWithChildren } from 'react'

type Toast = {
  id: number
  type: 'success' | 'error'
  message: string
}

type ConfirmOptions = {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  onConfirm: () => Promise<void> | void
}

type UIState = {
  toasts: Toast[]
  showSuccess: (message: string) => void
  showError: (message: string) => void
  removeToast: (id: number) => void
  confirm: (options: ConfirmOptions) => void
  confirmState: (ConfirmOptions & { open: boolean }) | null
  closeConfirm: () => void
}

const UIContext = createContext<UIState | undefined>(undefined)

let toastId = 0

export const UIProvider = ({ children }: PropsWithChildren) => {
  const [toasts, setToasts] = useState<Toast[]>([])
  const [confirmState, setConfirmState] = useState<(
    ConfirmOptions & { open: boolean }
  ) | null>(null)

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id))
  }, [])

  const showToast = useCallback(
    (type: Toast['type'], message: string) => {
      const id = ++toastId
      setToasts((prev) => [...prev, { id, type, message }])
      window.setTimeout(() => removeToast(id), 3000)
    },
    [removeToast],
  )

  const showSuccess = useCallback(
    (message: string) => showToast('success', message),
    [showToast],
  )

  const showError = useCallback(
    (message: string) => showToast('error', message),
    [showToast],
  )

  const confirm = useCallback((options: ConfirmOptions) => {
    setConfirmState({
      open: true,
      confirmText: '确认',
      cancelText: '取消',
      ...options,
    })
  }, [])

  const closeConfirm = useCallback(() => {
    setConfirmState(null)
  }, [])

  const value = useMemo(
    () => ({
      toasts,
      showSuccess,
      showError,
      removeToast,
      confirm,
      confirmState,
      closeConfirm,
    }),
    [toasts, showSuccess, showError, removeToast, confirm, confirmState, closeConfirm],
  )

  return <UIContext.Provider value={value}>{children}</UIContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export const useUIContext = () => {
  const context = useContext(UIContext)
  if (!context) {
    throw new Error('useUIContext must be used within UIProvider')
  }
  return context
}
