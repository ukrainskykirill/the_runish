import { useState } from 'react';
import { api } from '../../api/client';
import { useAuth } from '../../context/AuthContext';
import { useTelegram, requestTelegramContact, requestTelegramWriteAccess } from '../../hooks/useTelegram';
import { CheckIcon, InfoIcon, TelegramIcon } from '../../components/icons';

export function AuthGate() {
  const tg = useTelegram();
  const { refresh } = useAuth();
  const [authorizing, setAuthorizing] = useState(false);
  const [authorized, setAuthorized] = useState(false);
  const [notifyGranted, setNotifyGranted] = useState(false);
  const [sharingPhone, setSharingPhone] = useState(false);
  const [phoneShared, setPhoneShared] = useState(false);
  const [phoneWaiting, setPhoneWaiting] = useState(false);

  const insideTelegram = !!tg && !!tg.initData;

  async function handleAuthorize() {
    if (!tg) return;
    setAuthorizing(true);
    try {
      // Один явный блок: авторизация сразу же запрашивает разрешение писать в бота.
      const writeGranted = await requestTelegramWriteAccess(tg);
      await api.authTelegramWebApp(tg.initData, writeGranted);
      setAuthorized(true);
      setNotifyGranted(writeGranted);
      await refresh();
    } catch {
      setAuthorized(false);
    } finally {
      setAuthorizing(false);
    }
  }

  async function handleShareContact() {
    if (!tg) return;
    setSharingPhone(true);
    try {
      const granted = await requestTelegramContact(tg);
      if (!granted) return;
      // Сам номер Telegram присылает боту отдельным Update (не в браузер) —
      // ждём, пока воркер его обработает, и подтверждаем по /api/me.
      setPhoneWaiting(true);
      for (let i = 0; i < 10; i++) {
        await new Promise((resolve) => window.setTimeout(resolve, 1500));
        const me = await api.me();
        if (me.user?.phone) {
          setPhoneShared(true);
          await refresh();
          break;
        }
      }
    } finally {
      setSharingPhone(false);
      setPhoneWaiting(false);
    }
  }

  async function handleDevLogin() {
    setAuthorizing(true);
    try {
      await api.authDev();
      await refresh();
    } finally {
      setAuthorizing(false);
    }
  }

  return (
    <div className="mini-auth">
      <div className="mini-auth-sl speedlines" />
      <div className="mini-auth-card">
        <div className="eb">
          <span className="dot" />
          Telegram Mini App
        </div>
        <h1 className="m-h">Клуб в кармане</h1>
        <p className="mini-auth-lead">
          Авторизуйтесь через Telegram, чтобы открыть расписание, абонементы и профиль — без
          повторного входа на сайте.
        </p>

        {!insideTelegram ? (
          <div className="mini-auth-block">
            <div className="mab-title">
              <InfoIcon className="i i-sm" />
              Откройте в Telegram
            </div>
            <p className="mab-desc">
              Мини-приложение работает внутри Telegram — откройте его через кнопку меню бота.
            </p>
            {import.meta.env.DEV ? (
              <button className="btn btn-ghost btn-block" onClick={handleDevLogin} disabled={authorizing}>
                Войти как тестовый пользователь (dev)
              </button>
            ) : null}
          </div>
        ) : (
          <>
            <div className="mini-auth-block">
              <div className="mab-title">
                <TelegramIcon className="i i-sm" />
                Авторизация
              </div>
              <p className="mab-desc">
                Вход по вашему Telegram-аккаунту и разрешение на уведомления от бота (напоминания о
                тренировках и оплате). Заодно авторизует и на сайте.
              </p>
              <button
                className="btn btn-primary btn-block"
                onClick={handleAuthorize}
                disabled={authorizing || authorized}
              >
                {authorized
                  ? notifyGranted
                    ? 'Вы авторизованы, уведомления включены'
                    : 'Вы авторизованы'
                  : authorizing
                    ? 'Авторизация…'
                    : 'Авторизоваться и разрешить уведомления'}
              </button>
            </div>

            {authorized ? (
              <div className="mini-auth-block">
                <div className="mab-title">
                  <CheckIcon className="i i-sm" />
                  Поделиться номером
                </div>
                <p className="mab-desc">Нужен для оплаты и чека — не выходя из приложения.</p>
                <button
                  className="btn btn-ghost btn-block"
                  onClick={handleShareContact}
                  disabled={sharingPhone || phoneShared}
                >
                  {phoneShared
                    ? 'Номер сохранён'
                    : phoneWaiting
                      ? 'Сохраняем номер…'
                      : sharingPhone
                        ? 'Ждём подтверждения…'
                        : 'Поделиться номером'}
                </button>
              </div>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}
