import { useAuth } from '../../context/AuthContext';
import { useCart } from '../../context/CartContext';
import { useUI } from '../../context/UIContext';
import { CloseIcon } from '../icons';
import { LoginPanel } from './LoginPanel';

export function LoginModal() {
  const { loginModal, closeLoginModal, showToast } = useUI();
  const { refresh: refreshAuth } = useAuth();
  const { add: cartAdd } = useCart();

  if (!loginModal.open) return null;

  async function handleSuccess() {
    await refreshAuth();
    closeLoginModal();
    showToast('Вы вошли через Telegram');
    if (loginModal.pendingServiceId) {
      await cartAdd(loginModal.pendingServiceId);
    }
  }

  return (
    <div
      className="modal-bg"
      onClick={(e) => {
        if (e.target === e.currentTarget) closeLoginModal();
      }}
    >
      <div className="modal">
        <button className="modal-close" onClick={closeLoginModal} aria-label="Закрыть">
          <CloseIcon className="i i-sm" />
        </button>
        <LoginPanel reason={loginModal.reason} onSuccess={handleSuccess} showCancel onCancel={closeLoginModal} />
      </div>
    </div>
  );
}
