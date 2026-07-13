import { useEffect, useMemo } from 'react';

interface TelegramWebAppUser {
  id: number;
  first_name: string;
  last_name?: string;
  username?: string;
}

interface TelegramThemeParams {
  bg_color?: string;
  text_color?: string;
  hint_color?: string;
  link_color?: string;
  button_color?: string;
  button_text_color?: string;
  secondary_bg_color?: string;
}

interface TelegramMainButton {
  text: string;
  isVisible: boolean;
  isActive: boolean;
  setText(text: string): TelegramMainButton;
  onClick(cb: () => void): TelegramMainButton;
  offClick(cb: () => void): TelegramMainButton;
  show(): TelegramMainButton;
  hide(): TelegramMainButton;
  enable(): TelegramMainButton;
  disable(): TelegramMainButton;
  showProgress(leaveActive?: boolean): TelegramMainButton;
  hideProgress(): TelegramMainButton;
}

interface TelegramBackButton {
  isVisible: boolean;
  onClick(cb: () => void): TelegramBackButton;
  offClick(cb: () => void): TelegramBackButton;
  show(): TelegramBackButton;
  hide(): TelegramBackButton;
}

interface TelegramHapticFeedback {
  impactOccurred(style: 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'): void;
  notificationOccurred(type: 'error' | 'success' | 'warning'): void;
  selectionChanged(): void;
}

export interface TelegramWebApp {
  initData: string;
  initDataUnsafe: {
    user?: TelegramWebAppUser;
    start_param?: string;
  };
  themeParams: TelegramThemeParams;
  colorScheme: 'light' | 'dark';
  isExpanded: boolean;
  viewportHeight: number;
  MainButton: TelegramMainButton;
  BackButton: TelegramBackButton;
  HapticFeedback: TelegramHapticFeedback;
  ready(): void;
  expand(): void;
  close(): void;
  openLink(url: string, options?: { try_instant_view?: boolean }): void;
  // Оба метода отдают в колбэк ТОЛЬКО boolean — сам номер телефона Telegram
  // присылает боту отдельным сервисным сообщением (см. handleContact на бэкенде),
  // а не возвращает в браузер. https://core.telegram.org/bots/webapps
  requestContact(callback: (granted: boolean) => void): void;
  requestWriteAccess(callback: (granted: boolean) => void): void;
  onEvent(event: string, cb: () => void): void;
  offEvent(event: string, cb: () => void): void;
}

declare global {
  interface Window {
    Telegram?: {
      WebApp: TelegramWebApp;
    };
  }
}

function getWebApp(): TelegramWebApp | null {
  return typeof window !== 'undefined' && window.Telegram ? window.Telegram.WebApp : null;
}

/** true, если страница открыта внутри Telegram (initData присутствует). */
export function isInsideTelegram(): boolean {
  const wa = getWebApp();
  return !!wa && !!wa.initData;
}

/**
 * Обёртка над window.Telegram.WebApp. Возвращает null вне Telegram —
 * экраны mini app должны обрабатывать это состояние сами (см. AuthGate).
 */
export function useTelegram(): TelegramWebApp | null {
  const wa = useMemo(() => getWebApp(), []);

  useEffect(() => {
    if (!wa) return;
    wa.ready();
    wa.expand();
  }, [wa]);

  return wa;
}

/**
 * Промисифицированный requestContact(). Возвращает только факт согласия — номер
 * телефона Telegram доставит боту отдельным Update, backend сохранит его туда же,
 * куда и раньше (см. handleContact); вызывающий код должен после этого поллить /api/me.
 */
export function requestTelegramContact(wa: TelegramWebApp): Promise<boolean> {
  return new Promise((resolve) => {
    wa.requestContact((granted) => resolve(granted));
  });
}

/** Промисифицированный requestWriteAccess(). */
export function requestTelegramWriteAccess(wa: TelegramWebApp): Promise<boolean> {
  return new Promise((resolve) => {
    wa.requestWriteAccess((granted) => resolve(granted));
  });
}
