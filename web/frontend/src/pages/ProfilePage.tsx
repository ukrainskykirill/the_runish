import { useEffect } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { formatDate, formatPrice } from '../api/client';
import type { Order } from '../api/types';
import { useAuth } from '../context/AuthContext';

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
  const { user, subscriptions, orders, loading } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!loading && !user) navigate('/', { replace: true });
  }, [loading, user, navigate]);

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
