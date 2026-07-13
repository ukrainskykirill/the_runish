import type {
  CartResponse,
  CheckoutResponse,
  MeResponse,
  CatalogResponse,
  HomeResponse,
  MerchItem,
  NewsItem,
  TelegramStartResponse,
  Training,
  TrainingOccurrence,
  TrainingRegistration,
  SurveyResponse,
  SurveyAnswers,
} from './types';

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string) {
    super(code);
    this.status = status;
    this.code = code;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, {
    credentials: 'same-origin',
    headers: init?.body ? { 'Content-Type': 'application/json' } : undefined,
    ...init,
  });

  if (!res.ok) {
    let code = 'unknown_error';
    try {
      const data = await res.json();
      if (data && typeof data.error === 'string') code = data.error;
    } catch {
      // ignore non-JSON error bodies
    }
    throw new ApiError(res.status, code);
  }

  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

const cache = new Map<string, { expiresAt: number; data: unknown }>();

async function cachedRequest<T>(path: string, signal?: AbortSignal, ttlMs = 60_000): Promise<T> {
  const now = Date.now();
  const cached = cache.get(path);
  if (cached && cached.expiresAt > now) return cached.data as T;

  const data = await request<T>(path, { signal });
  cache.set(path, { expiresAt: now + ttlMs, data });
  return data;
}

export const api = {
  me: (signal?: AbortSignal) => request<MeResponse>('/me', { signal }),
  setPhone: (phone: string) =>
    request<{ phone: string }>('/me/phone', {
      method: 'POST',
      body: JSON.stringify({ phone }),
    }),
  home: (signal?: AbortSignal) => request<HomeResponse>('/home', { signal }),
  catalog: (signal?: AbortSignal) => request<CatalogResponse>('/catalog', { signal }),
  news: (signal?: AbortSignal) => cachedRequest<{ news: NewsItem[] }>('/news', signal),
  newsById: (id: number, signal?: AbortSignal) => cachedRequest<NewsItem>(`/news/${id}`, signal),
  merch: (signal?: AbortSignal) => cachedRequest<{ merch: MerchItem[] }>('/merch', signal),
  schedule: (signal?: AbortSignal) => cachedRequest<{ trainings: Training[] }>('/schedule', signal),
  trainingsUpcoming: (signal?: AbortSignal) =>
    request<{ occurrences: TrainingOccurrence[] }>('/trainings/upcoming', { signal }),
  trainingRegister: (trainingId: number, sessionDate: string) =>
    request<TrainingRegistration>('/trainings/register', {
      method: 'POST',
      body: JSON.stringify({ training_id: trainingId, session_date: sessionDate }),
    }),
  trainingCancel: (registrationId: number) =>
    request<{ ok: boolean }>('/trainings/cancel', {
      method: 'POST',
      body: JSON.stringify({ registration_id: registrationId }),
    }),

  cart: () => request<CartResponse>('/cart'),
  cartAdd: (serviceId: number) =>
    request<CartResponse>('/cart', {
      method: 'POST',
      body: JSON.stringify({ service_id: serviceId }),
    }),
  cartRemove: (serviceId: number) =>
    request<CartResponse>('/cart/remove', {
      method: 'POST',
      body: JSON.stringify({ service_id: serviceId }),
    }),

  checkout: () => request<CheckoutResponse>('/checkout', { method: 'POST' }),

  survey: (signal?: AbortSignal) => request<SurveyResponse>('/survey', { signal }),
  surveySubmit: (answers: SurveyAnswers) =>
    request<{ status: string }>('/survey', {
      method: 'POST',
      body: JSON.stringify({ answers }),
    }),
  surveyProgress: (answers: SurveyAnswers) =>
    request<{ status: string }>('/survey/progress', {
      method: 'POST',
      body: JSON.stringify({ answers }),
    }),

  authTelegramStart: () => request<TelegramStartResponse>('/auth/telegram/start'),
  authTelegramStatus: (nonce: string) =>
    request<{ status: string }>(`/auth/telegram/status?nonce=${encodeURIComponent(nonce)}`),
  authTelegramComplete: (nonce: string) =>
    request<{ ok: boolean }>(`/auth/telegram/complete?nonce=${encodeURIComponent(nonce)}`),
  authTelegramWebApp: (initData: string, allowWrite: boolean) =>
    request<{ ok: boolean }>('/auth/telegram/webapp', {
      method: 'POST',
      body: JSON.stringify({ init_data: initData, allow_write: allowWrite }),
    }),
  enableNotifications: () => request<{ ok: boolean }>('/me/notifications/enable', { method: 'POST' }),
  /** Только для локальной разработки: включён на бэке, только если DEV_AUTH_BYPASS=1. */
  authDev: () => request<{ ok: boolean }>('/auth/dev', { method: 'POST', body: '{}' }),
  logout: () => request<{ ok: boolean }>('/auth/logout', { method: 'POST' }),
};

/** Переводит копейки в строку вида "5 900 ₽" / "0 ₽". */
export function formatPrice(kop: number): string {
  const rub = Math.round(kop / 100);
  if (rub === 0) return '0 ₽';
  return `${rub.toLocaleString('ru-RU')} ₽`;
}

/** Форматирует дату ISO в "12 июня 2026". */
export function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' });
}
