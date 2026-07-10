import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import type { TelegramStartResponse } from '../../api/types';
import { useAuth } from '../../context/AuthContext';
import { CheckIcon, InfoIcon, TelegramIcon, VKIcon } from '../icons';

interface LoginPanelProps {
  reason?: string | null;
  onSuccess: () => void;
  showCancel?: boolean;
  onCancel?: () => void;
}

export function LoginPanel({ reason, onSuccess, showCancel, onCancel }: LoginPanelProps) {
  const [start, setStart] = useState<TelegramStartResponse | null>(null);
  const completingRef = useRef(false);
  const { vkEnabled } = useAuth();

  useEffect(() => {
    api.authTelegramStart().then(setStart).catch(() => setStart({ bot_username: '', nonce: '' }));
  }, []);

  useEffect(() => {
    if (!start?.nonce) return;
    const interval = setInterval(async () => {
      try {
        const { status } = await api.authTelegramStatus(start.nonce);
        if (status === 'confirmed' && !completingRef.current) {
          completingRef.current = true;
          await api.authTelegramComplete(start.nonce);
          onSuccess();
        }
      } catch {
        // игнорируем ошибки поллинга, попробуем на следующем тике
      }
    }, 2000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [start]);

  return (
    <>
      <div className="modal-top">
        <div className="modal-sl speedlines" />
        <div className="tgring">
          <TelegramIcon style={{ width: 32, height: 32 }} />
        </div>
        <h3>Вход через Telegram</h3>
        <p>Откройте бота в Telegram и нажмите «Старт» — вход произойдёт автоматически.</p>
      </div>
      <div className="modal-body">
        {reason ? (
          <div className="modal-note">
            <InfoIcon className="i i-sm" />
            {reason}
          </div>
        ) : null}
        <div className="modal-perm">
          <div>
            <CheckIcon className="i i-sm" />
            Имя и фото профиля Telegram
          </div>
          <div>
            <CheckIcon className="i i-sm" />
            Напоминания о тренировках и оплате
          </div>
        </div>
        {start?.bot_username ? (
          <a
            className="btn btn-primary btn-block"
            href={`https://t.me/${start.bot_username}?start=${start.nonce}`}
            target="_blank"
            rel="noopener noreferrer"
          >
            Открыть в Telegram
          </a>
        ) : (
          <div className="modal-note">
            <InfoIcon className="i i-sm" />
            Вход временно недоступен. Попробуйте позже.
          </div>
        )}
        {vkEnabled ? (
          <a className="btn btn-vk btn-block" href="/api/auth/vk/start">
            <VKIcon className="i i-sm" />
            Войти через ВКонтакте
          </a>
        ) : null}
        {showCancel ? (
          <div className="modal-actions">
            <button className="btn btn-ghost btn-sm" onClick={onCancel}>
              Отмена
            </button>
          </div>
        ) : null}
      </div>
    </>
  );
}
