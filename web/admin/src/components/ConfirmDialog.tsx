import { useUIContext } from '../context/UIContext'

const ConfirmDialog = () => {
  const { confirmState, closeConfirm } = useUIContext()

  if (!confirmState?.open) {
    return null
  }

  const { title, message, confirmText = '确认', cancelText = '取消', onConfirm } =
    confirmState

  const handleConfirm = async () => {
    try {
      await onConfirm()
    } finally {
      closeConfirm()
    }
  }

  return (
    <div className="dialog-backdrop" role="alertdialog" aria-modal>
      <div className="dialog">
        <h2 className="dialog__title">{title}</h2>
        <p style={{ color: '#475467', margin: '0 0 12px' }}>{message}</p>
        <div className="dialog__actions">
          <button
            type="button"
            className="button button--ghost"
            onClick={closeConfirm}
          >
            {cancelText}
          </button>
          <button type="button" className="button" onClick={handleConfirm}>
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  )
}

export default ConfirmDialog
