import { useState } from 'react';
import { api, formatDate } from '../../api/client';
import { useAuth } from '../../context/AuthContext';
import { useUI } from '../../context/UIContext';
import { useTelegram, requestTelegramContact } from '../../hooks/useTelegram';
import { TELEGRAM_LINKS } from '../../lib/constants';
import { CheckIcon, SupportIcon, TelegramIcon } from '../../components/icons';

const MONTHS_GEN = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
];

function formatTrainingDate(date: string): string {
  const [, m, d] = date.split('-').map(Number);
  return `${d} ${MONTHS_GEN[m - 1]}`;
}

function initials(fullName: string): string {
  const parts = fullName.trim().split(/\s+/).filter(Boolean);
  return parts.slice(0, 2).map((p) => p[0]?.toUpperCase() ?? '').join('') || '?';
}

export function MiniProfilePage() {
  const { user, subscriptions, trainingRegistrations, refresh, logout } = useAuth();
  const { showToast } = useUI();
  const tg = useTelegram();
  const [cancelingId, setCancelingId] = useState<number | null>(null);
  const [sharingPhone, setSharingPhone] = useState(false);
  const [phoneWaiting, setPhoneWaiting] = useState(false);

  if (!user) return null;

  async function cancelTraining(regId: number) {
    setCancelingId(regId);
    try {
      await api.trainingCancel(regId);
      showToast('Запись отменена');
      await refresh();
    } catch {
      showToast('Не удалось отменить запись');
    } finally {
      setCancelingId(null);
    }
  }

  async function handleShareContact() {
    if (!tg) return;
    setSharingPhone(true);
    try {
      const granted = await requestTelegramContact(tg);
      if (!granted) return;
      setPhoneWaiting(true);
      for (let i = 0; i < 10; i++) {
        await new Promise((resolve) => window.setTimeout(resolve, 1500));
        const me = await api.me();
        if (me.user?.phone) {
          showToast('Номер сохранён');
          await refresh();
          break;
        }
      }
    } finally {
      setSharingPhone(false);
      setPhoneWaiting(false);
    }
  }

  return (
    <div className="pad stack">
      <div className="prof-head">
        <div className="pav">{initials(user.full_name)}</div>
        <div>
          <div className="pn">{user.full_name || 'Без имени'}</div>
          {user.username ? (
            <div className="ph">
              <TelegramIcon className="i i-xs" />@{user.username}
            </div>
          ) : null}
        </div>
      </div>

      {!user.phone ? (
        <div className="row-card">
          <span className="ri" style={{ background: 'var(--runish-red)', color: 'var(--cream-light)', borderColor: 'transparent' }}>
            <CheckIcon className="i i-sm" />
          </span>
          <div className="rt">
            <div className="a">Номер телефона</div>
            <div className="b">Нужен для оплаты и чека</div>
          </div>
          <button className="pill-btn" disabled={sharingPhone} onClick={handleShareContact}>
            {phoneWaiting ? 'Ждём…' : sharingPhone ? 'Ждём…' : 'Поделиться'}
          </button>
        </div>
      ) : null}

      {subscriptions.length > 0 ? (
        <div className="sub-status">
          <div className="top">
            <div>
              <div className="lab">Мой абонемент</div>
              <div className="name">{subscriptions[0].service_title}</div>
            </div>
            <span className="chip chip-track">Активен</span>
          </div>
          <div className="until">
            Действует до <b>{formatDate(subscriptions[0].expires_at)}</b>
          </div>
        </div>
      ) : null}

      {trainingRegistrations.length > 0 ? (
        <div>
          <div className="sec-top">
            <h3 className="m-h">Мои записи</h3>
          </div>
          <div className="stack">
            {trainingRegistrations.map((reg) => (
              <div className="row-card" key={reg.id}>
                <div className="rt">
                  <div className="a">{reg.title}</div>
                  <div className="b">
                    {formatTrainingDate(reg.session_date)} в {reg.start_time} · {reg.place}
                  </div>
                </div>
                <button
                  className="pill-btn"
                  disabled={cancelingId === reg.id}
                  onClick={() => cancelTraining(reg.id)}
                >
                  Отменить
                </button>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      <div>
        <div className="sec-top">
          <h3 className="m-h">Клуб на связи</h3>
        </div>
        <div className="stack">
          <a className="row-card" href={TELEGRAM_LINKS.channel} target="_blank" rel="noopener noreferrer">
            <span className="ri" style={{ background: 'var(--runish-red)', color: 'var(--cream-light)', borderColor: 'transparent' }}>
              <TelegramIcon className="i i-sm" />
            </span>
            <div className="rt">
              <div className="a">Канал клуба</div>
              <div className="b">Анонсы, фото и жизнь клуба</div>
            </div>
          </a>
          <a className="row-card" href={TELEGRAM_LINKS.support} target="_blank" rel="noopener noreferrer">
            <span className="ri" style={{ background: 'var(--runish-red)', color: 'var(--cream-light)', borderColor: 'transparent' }}>
              <SupportIcon className="i i-sm" />
            </span>
            <div className="rt">
              <div className="a">Поддержка</div>
              <div className="b">Вопросы по абонементу и оплате</div>
            </div>
          </a>
        </div>
      </div>

      <button className="btn btn-ghost btn-block" onClick={logout}>
        Выйти
      </button>
    </div>
  );
}
