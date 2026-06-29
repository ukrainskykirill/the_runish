import { Link } from 'react-router-dom';
import { formatDate } from '../../api/client';
import { useAuth } from '../../context/AuthContext';
import { useUI } from '../../context/UIContext';
import { TELEGRAM_LINKS } from '../../lib/constants';
import { getInitials } from '../../lib/format';
import { FlagIcon, ListIcon, LogoutIcon, SupportIcon, UserIcon } from '../icons';

interface AccountDropdownProps {
  open: boolean;
  onClose: () => void;
}

export function AccountDropdown({ open, onClose }: AccountDropdownProps) {
  const { user, subscriptions, logout } = useAuth();
  const { showToast } = useUI();

  if (!user) return null;

  const displayName = user.full_name || user.username || 'Бегун';
  const activeSub = subscriptions[0];

  async function handleLogout() {
    await logout();
    onClose();
    showToast('Вы вышли');
  }

  return (
    <div className={`dd ${open ? 'open' : ''}`}>
      <div className="pf-top">
        <div className="pf-sl speedlines" />
        <div className="av">{getInitials(displayName)}</div>
        <div>
          <div className="nm">{displayName}</div>
          {user.username ? <div className="hd">@{user.username}</div> : null}
        </div>
      </div>
      {activeSub ? (
        <div className="sub-card">
          <div className="lab">Текущая подписка</div>
          <div className="row">
            <span className="nm">{activeSub.service_title}</span>
            <span className="chip chip-track">Активен</span>
          </div>
          <div className="until">
            Действует до <b>{formatDate(activeSub.expires_at)}</b>
          </div>
        </div>
      ) : null}
      <div className="pf-links">
        <Link className="pf-link" to="/me" onClick={onClose}>
          <UserIcon className="i i-sm" />
          Личный кабинет
        </Link>
        <Link className="pf-link" to="/runners" onClick={onClose}>
          <ListIcon className="i i-sm" />
          Пробежки и подписки
        </Link>
        <Link className="pf-link" to="/news" onClick={onClose}>
          <FlagIcon className="i i-sm" />
          Новости клуба
        </Link>
        <a className="pf-link" href={TELEGRAM_LINKS.support} target="_blank" rel="noopener noreferrer">
          <SupportIcon className="i i-sm" />
          Поддержка
        </a>
        <button className="pf-link out" onClick={handleLogout}>
          <LogoutIcon className="i i-sm" />
          Выйти
        </button>
      </div>
    </div>
  );
}
