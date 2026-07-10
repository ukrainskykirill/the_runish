import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, ApiError } from '../../api/client';
import type { TrainingOccurrence } from '../../api/types';
import { useAuth } from '../../context/AuthContext';
import { useUI } from '../../context/UIContext';
import { useAsync } from '../../lib/useAsync';
import { ErrorState } from '../LoadState';
import { EntryFeeNotice } from '../EntryFeeNotice';

const DOW = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
const MONTHS_GEN = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
];

const ERR_MESSAGES: Record<string, string> = {
  entry_fee_required: 'Сначала оплатите вступительный взнос',
  access_required: 'Нужна активная подписка или разовая тренировка',
  session_full: 'Свободных мест больше нет',
  already_registered: 'Вы уже записаны на это занятие',
  session_cancelled: 'Занятие отменено',
  past_session: 'Запись на это занятие закрыта',
  not_found: 'Тренировка не найдена',
};

/** "2026-07-02" + weekday → "Чт, 2 июля". */
function formatSessionDate(date: string, weekday: number): string {
  const [, m, d] = date.split('-').map(Number);
  return `${DOW[weekday - 1]}, ${d} ${MONTHS_GEN[m - 1]}`;
}

function occKey(o: TrainingOccurrence): string {
  return `${o.training_id}|${o.session_date}`;
}

export function TrainingSignup() {
  const { user, refresh } = useAuth();
  const { openLoginModal, showToast } = useUI();
  const [reloadKey, setReloadKey] = useState(0);
  const [busyKey, setBusyKey] = useState<string | null>(null);

  const { data, loading, error } = useAsync(
    () => api.trainingsUpcoming(),
    [reloadKey, user?.id],
  );
  const occurrences = data?.occurrences ?? [];

  async function reload() {
    setReloadKey((k) => k + 1);
    await refresh();
  }

  async function handleRegister(o: TrainingOccurrence) {
    if (!user) {
      openLoginModal({ reason: 'training' });
      return;
    }
    setBusyKey(occKey(o));
    try {
      await api.trainingRegister(o.training_id, o.session_date);
      showToast('Вы записаны на тренировку');
      await reload();
    } catch (e) {
      const code = e instanceof ApiError ? e.code : 'unknown_error';
      if (code === 'login_required') openLoginModal({ reason: 'training' });
      else showToast(ERR_MESSAGES[code] ?? 'Не удалось записаться');
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

  if (loading) {
    return (
      <div className="ts-loading">
        {Array.from({ length: 4 }, (_, i) => (
          <span className="sk" key={i} />
        ))}
      </div>
    );
  }
  if (error) return <ErrorState message="Не удалось загрузить занятия для записи" />;

  if (occurrences.length === 0) {
    return <div className="empty-state">Ближайших тренировок для записи пока нет</div>;
  }

  // Группируем по дате.
  const groups: { date: string; weekday: number; items: TrainingOccurrence[] }[] = [];
  for (const o of occurrences) {
    const last = groups[groups.length - 1];
    if (last && last.date === o.session_date) last.items.push(o);
    else groups.push({ date: o.session_date, weekday: o.weekday, items: [o] });
  }

  return (
    <div className="ts">
      {user && !user.entry_fee_paid && <EntryFeeNotice />}

      {groups.map((g) => (
        <div className="ts-group" key={g.date}>
          <div className="ts-date">{formatSessionDate(g.date, g.weekday)}</div>
          <div className="ts-cards">
            {g.items.map((o) => (
              <TrainingCard
                key={occKey(o)}
                occ={o}
                user={user}
                busy={busyKey === occKey(o)}
                onRegister={() => handleRegister(o)}
                onCancel={() => handleCancel(o)}
              />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

interface CardProps {
  occ: TrainingOccurrence;
  user: ReturnType<typeof useAuth>['user'];
  busy: boolean;
  onRegister: () => void;
  onCancel: () => void;
}

function TrainingCard({ occ, user, busy, onRegister, onCancel }: CardProps) {
  const spotsLeft =
    occ.capacity != null ? Math.max(occ.capacity - occ.registered_count, 0) : null;

  return (
    <div className={'tcard' + (occ.registered ? ' is-reg' : '') + (occ.cancelled ? ' is-cancelled' : '')}>
      <div className="tcard-main">
        <div className="tcard-title">{occ.title}</div>
        <div className="tcard-meta">
          <span className="tcard-time">{occ.start_time}</span>
          {occ.place_url ? (
            <a className="tcard-place tcard-place-link" href={occ.place_url} target="_blank" rel="noopener noreferrer">{occ.place}</a>
          ) : (
            <span className="tcard-place">{occ.place}</span>
          )}
        </div>
        <div className="tcard-info">
          {occ.cancelled ? (
            <span className="tcard-note red">Занятие отменено</span>
          ) : (
            <>
              {spotsLeft != null && (
                <span className={'tcard-note' + (spotsLeft === 0 ? ' red' : '')}>
                  {spotsLeft === 0 ? 'Мест нет' : `Осталось мест: ${spotsLeft}`}
                </span>
              )}
              {user && occ.registered && occ.quota_left != null && (
                <span className="tcard-note">По абонементу осталось: {occ.quota_left}</span>
              )}
            </>
          )}
        </div>
      </div>
      <div className="tcard-action">{renderAction(occ, user, busy, spotsLeft, onRegister, onCancel)}</div>
    </div>
  );
}

function renderAction(
  occ: TrainingOccurrence,
  user: ReturnType<typeof useAuth>['user'],
  busy: boolean,
  spotsLeft: number | null,
  onRegister: () => void,
  onCancel: () => void,
) {
  if (occ.registered) {
    return (
      <div className="tcard-reg">
        <span className="chip chip-track">Вы записаны</span>
        <button className="btn btn-ghost btn-sm" disabled={busy} onClick={onCancel}>
          Отменить
        </button>
      </div>
    );
  }
  if (occ.cancelled) {
    return <button className="btn btn-ghost btn-sm" disabled>Недоступно</button>;
  }
  if (!user) {
    return (
      <button className="btn btn-primary btn-sm" disabled={busy} onClick={onRegister}>
        Записаться
      </button>
    );
  }
  if (!user.entry_fee_paid) {
    return (
      <Link className="btn btn-ghost btn-sm" to="/runners">
        Нужен взнос
      </Link>
    );
  }
  if (!occ.in_my_access) {
    return (
      <Link className="btn btn-ghost btn-sm" to="/runners">
        Нужен доступ
      </Link>
    );
  }
  if (spotsLeft === 0) {
    return <button className="btn btn-ghost btn-sm" disabled>Мест нет</button>;
  }
  return (
    <button className="btn btn-primary btn-sm" disabled={busy} onClick={onRegister}>
      Записаться
    </button>
  );
}
