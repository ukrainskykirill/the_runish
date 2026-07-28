import { useState } from 'react';
import { api, ApiError } from '../../api/client';
import type { TrainingOccurrence } from '../../api/types';
import { useAuth } from '../../context/AuthContext';
import { useUI } from '../../context/UIContext';
import { useAsync } from '../../lib/useAsync';
import { ErrorState } from '../../components/LoadState';
import { CheckIcon, ClockIcon, PinIcon, PlusIcon } from '../../components/icons';

const DOW = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

const ERR_MESSAGES: Record<string, string> = {
  entry_fee_required: 'Сначала оплатите вступительный взнос',
  access_required: 'Нужна активная подписка',
  session_full: 'Свободных мест больше нет',
  already_registered: 'Вы уже записаны на это занятие',
  session_cancelled: 'Занятие отменено',
  past_session: 'Запись на это занятие закрыта',
  not_found: 'Тренировка не найдена',
};

function isoWeekday(d: Date): number {
  return ((d.getDay() + 6) % 7) + 1;
}

function parseSessionDate(date: string): Date {
  const [y, m, d] = date.split('-').map(Number);
  return new Date(y, m - 1, d);
}

function occKey(o: TrainingOccurrence): string {
  return `${o.training_id}|${o.session_date}`;
}

export function MiniSchedulePage() {
  const { user } = useAuth();
  const { showToast } = useUI();
  const [reloadKey, setReloadKey] = useState(0);
  const [busyKey, setBusyKey] = useState<string | null>(null);

  const { data, loading, error } = useAsync(() => api.trainingsUpcoming(), [reloadKey]);
  const occurrences = (data?.occurrences ?? []).filter((o) => !o.past);

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const todayIso = isoWeekday(today);
  const monday = new Date(today);
  monday.setDate(today.getDate() - (todayIso - 1));

  const daysWithSessions = new Set(occurrences.map((o) => o.session_date));
  const weekDays = Array.from({ length: 7 }, (_, i) => {
    const date = new Date(monday);
    date.setDate(monday.getDate() + i);
    const iso = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
    return { wd: i + 1, num: date.getDate(), isToday: iso === `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}-${String(today.getDate()).padStart(2, '0')}`, hasSession: daysWithSessions.has(iso) };
  });

  async function reload() {
    setReloadKey((k) => k + 1);
  }

  async function handleRegister(o: TrainingOccurrence) {
    setBusyKey(occKey(o));
    try {
      await api.trainingRegister(o.training_id, o.session_date);
      showToast('Вы записаны на тренировку');
      await reload();
    } catch (e) {
      const code = e instanceof ApiError ? e.code : 'unknown_error';
      showToast(ERR_MESSAGES[code] ?? 'Не удалось записаться');
    } finally {
      setBusyKey(null);
    }
  }

  async function handleCancel(o: TrainingOccurrence) {
    if (!o.my_registration_id) return;
    setBusyKey(occKey(o));
    try {
      await api.trainingCancel(o.my_registration_id);
      showToast('Запись отменена');
      await reload();
    } catch {
      showToast('Не удалось отменить запись');
    } finally {
      setBusyKey(null);
    }
  }

  return (
    <div className="pad stack">
      <div className="weekbar">
        {weekDays.map((d) => (
          <div key={d.wd} className={'wd' + (d.isToday ? ' on' : '') + (d.hasSession ? ' dot' : '')}>
            <div className="wn">{DOW[d.wd - 1]}</div>
            <div className="wnum">{d.num}</div>
          </div>
        ))}
      </div>

      {!user ? (
        <div className="empty-state">Авторизуйтесь, чтобы записаться на тренировку</div>
      ) : loading ? (
        <div className="ts-loading">
          {Array.from({ length: 3 }, (_, i) => (
            <span className="sk" key={i} />
          ))}
        </div>
      ) : error ? (
        <ErrorState message="Не удалось загрузить занятия" />
      ) : occurrences.length === 0 ? (
        <div className="empty-state">Ближайших тренировок для записи пока нет</div>
      ) : (
        occurrences.map((o) => {
          const date = parseSessionDate(o.session_date);
          const busy = busyKey === occKey(o);
          return (
            <div className="tr" key={occKey(o)}>
              <div className="td-date">
                <div className="d">{date.getDate()}</div>
                <div className="m">{DOW[o.weekday - 1]}</div>
              </div>
              <div className="vline" />
              <div className="ti">
                <div className="tt">
                  {o.title}
                  {o.kind === 'sunday_runish' ? <span className="chip chip-track">Sunday Runish</span> : null}
                </div>
                <div className="meta">
                  <span>
                    <ClockIcon className="i i-xs" />
                    {o.start_time}
                  </span>
                  <span>
                    <PinIcon className="i i-xs" />
                    {o.place}
                  </span>
                </div>
              </div>
              <div className="go">
                {o.registered ? (
                  <button className="pill-btn on" disabled={busy} onClick={() => handleCancel(o)}>
                    <CheckIcon className="i i-xs" />
                    Записан
                  </button>
                ) : (
                  <button className="pill-btn" disabled={busy || !o.available} onClick={() => handleRegister(o)}>
                    <PlusIcon className="i i-xs" />
                    Я приду
                  </button>
                )}
              </div>
            </div>
          );
        })
      )}
    </div>
  );
}
