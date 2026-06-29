import { useNavigate, useSearchParams } from 'react-router-dom';
import { LoginPanel } from '../components/auth/LoginPanel';
import { useAuth } from '../context/AuthContext';
import { useUI } from '../context/UIContext';

export function LoginPage() {
  const [params] = useSearchParams();
  const reason = params.get('reason');
  const navigate = useNavigate();
  const { refresh } = useAuth();
  const { showToast } = useUI();

  async function handleSuccess() {
    await refresh();
    showToast('Вы вошли через Telegram');
    navigate('/');
  }

  return (
    <section className="sec">
      <div className="wrap login-page-wrap">
        <div className="modal">
          <LoginPanel reason={reason} onSuccess={handleSuccess} />
        </div>
      </div>
    </section>
  );
}
