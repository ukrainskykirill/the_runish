import { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api, formatDate, formatPrice } from '../api/client';
import type { Order, PlanResponse, SurveyResponse } from '../api/types';
import { WeekPlanView } from '../components/plan/WeekPlanView';
import { useAuth } from '../context/AuthContext';
import { useUI } from '../context/UIContext';
import { useAsync } from '../lib/useAsync';

const MONTHS_GEN = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
];

/** "2026-07-02" → "2 июля". */
function formatTrainingDate(date: string): string {
  const [, m, d] = date.split('-').map(Number);
  return `${d} ${MONTHS_GEN[m - 1]}`;
}

function orderStatusChip(status: Order['status']): { label: string; className: string } {
  switch (status) {
    case 'paid':
      return { label: 'Оплачен', className: 'chip chip-track' };
    case 'cancelled':
      return { label: 'Отменён', className: 'chip chip-red' };
    default:
      return { label: 'Создан', className: 'chip' };
  }
}

export function ProfilePage() {
  const { user, subscriptions, orders, trainingRegistrations, loading, refresh } = useAuth();
  const { showToast } = useUI();
  const navigate = useNavigate();
  const [cancelingId, setCancelingId] = useState<number | null>(null);
  const survey = useAsync<SurveyResponse>((signal) => api.survey(signal), []);
  const surveyDone = survey.data?.status === 'completed';
  // План — часть подписки: без активной подписки секцию не показываем и не грузим.
  const hasSub = subscriptions.length > 0;
  const plan = useAsync<PlanResponse | null>(
    (signal) => (hasSub ? api.plan(undefined, signal) : Promise.resolve(null)),
    [hasSub],
  );

  useEffect(() => {
    if (!loading && !user) navigate('/', { replace: true });
  }, [loading, user, navigate]);

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

  if (loading || !user) return null;

  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Аккаунт</div>
          <h1 className="d2">Личный кабинет</h1>
        </div>
      </section>

      <section className="sec">
        <div className="wrap">
          <div className="sec-head">
            <div className="eb">Анкета</div>
            <h2 className="d2">Анкета бегуна</h2>
          </div>
          <div className="sub-card" style={{ maxWidth: 560 }}>
            <div className="lab">Знакомство</div>
            <div className="row">
              <span className="nm">
                {surveyDone ? 'Анкета заполнена' : 'Анкета не пройдена'}
              </span>
              <span className={`chip ${surveyDone ? 'chip-track' : 'chip-red'}`}>
                {surveyDone ? 'Готово' : 'Ждёт ответов'}
              </span>
            </div>
            <div className="until">
              {surveyDone
                ? 'Спасибо! Мы учтём ваши ответы при подборе формата.'
                : 'Пара минут — и мы подберём комфортный формат тренировок именно для вас.'}
            </div>
            <p style={{ marginTop: 16 }}>
              <Link className="btn btn-primary btn-sm" to="/survey">
                {surveyDone ? 'Изменить ответы' : 'Пройти анкету'}
              </Link>
            </p>
          </div>
        </div>
      </section>

      <section className="sec alt">
        <div className="wrap">
          <div className="sec-head">
            <div className="eb">Подписки</div>
            <h2 className="d2">Активные подписки</h2>
          </div>
          {subscriptions.length === 0 ? (
            <div className="empty-state">
              <div>Нет активных подписок</div>
              <p style={{ marginTop: 16 }}>
                <Link className="btn btn-primary btn-sm" to="/runners">
                  Выбрать подписку
                </Link>
              </p>
            </div>
          ) : (
            <div className="profile-grid">
              {subscriptions.map((sub) => (
                <div className="sub-card" key={sub.id}>
                  <div className="lab">Подписка</div>
                  <div className="row">
                    <span className="nm">{sub.service_title}</span>
                    <span className="chip chip-track">Активен</span>
                  </div>
                  <div className="until">
                    Действует до <b>{formatDate(sub.expires_at)}</b>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      {hasSub && (
        <section className="sec">
          <div className="wrap">
            <div className="sec-head between">
              <div>
                <div className="eb">Подписка</div>
                <h2 className="d2">План тренировок</h2>
              </div>
              <Link className="btn btn-ghost btn-sm" to="/plan">
                Все недели
              </Link>
            </div>
            {plan.loading ? (
              <div className="empty-state">Загрузка…</div>
            ) : plan.error || !plan.data ? (
              <div className="empty-state">Не удалось загрузить план</div>
            ) : (
              <WeekPlanView groups={plan.data.groups} materials={plan.data.materials} />
            )}
          </div>
        </section>
      )}

      <section className="sec">
        <div className="wrap">
          <div className="sec-head">
            <div className="eb">Тренировки</div>
            <h2 className="d2">Мои записи</h2>
          </div>
          {trainingRegistrations.length === 0 ? (
            <div className="empty-state">
              <div>Нет записей на тренировки</div>
              <p style={{ marginTop: 16 }}>
                <Link className="btn btn-primary btn-sm" to="/schedule">
                  Записаться на тренировку
                </Link>
              </p>
            </div>
          ) : (
            <div className="profile-grid">
              {trainingRegistrations.map((reg) => (
                <div className="sub-card" key={reg.id}>
                  <div className="lab">Тренировка</div>
                  <div className="row">
                    <span className="nm">{reg.title}</span>
                    <span className="chip chip-track">Записан</span>
                  </div>
                  <div className="until">
                    {formatTrainingDate(reg.session_date)} в <b>{reg.start_time}</b> · {reg.place}
                  </div>
                  <p style={{ marginTop: 12 }}>
                    <button
                      className="btn btn-ghost btn-sm"
                      disabled={cancelingId === reg.id}
                      onClick={() => cancelTraining(reg.id)}
                    >
                      Отменить запись
                    </button>
                  </p>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>

      <section className="sec alt">
        <div className="wrap">
          <div className="sec-head">
            <div className="eb">История</div>
            <h2 className="d2">Заказы</h2>
          </div>
          {orders.length === 0 ? (
            <div className="empty-state">Заказов пока нет</div>
          ) : (
            <table className="orders-table">
              <thead>
                <tr>
                  <th>№</th>
                  <th>Дата</th>
                  <th>Сумма</th>
                  <th>Статус</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((order) => {
                  const chip = orderStatusChip(order.status);
                  return (
                    <tr key={order.id}>
                      <td>#{order.id}</td>
                      <td>{formatDate(order.created_at)}</td>
                      <td className="amount">{formatPrice(order.amount_kop)}</td>
                      <td>
                        <span className={chip.className}>{chip.label}</span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>
      </section>
    </>
  );
}
