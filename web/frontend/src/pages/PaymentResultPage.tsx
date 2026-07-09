import { useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useCart } from '../context/CartContext';

interface PaymentResultPageProps {
  status: 'success' | 'fail';
}

type View = 'pending' | 'paid' | 'failed';

export function PaymentResultPage({ status }: PaymentResultPageProps) {
  const { orders, refresh: refreshAuth } = useAuth();
  const { refresh: refreshCart } = useCart();
  const [settled, setSettled] = useState(status === 'fail');

  useEffect(() => {
    if (status === 'fail') return;
    let active = true;
    async function poll() {
      for (let i = 0; i < 8 && active; i++) {
        await Promise.all([refreshAuth(), refreshCart()]);
        if (i < 7) {
          await new Promise((resolve) => window.setTimeout(resolve, 1500));
        }
      }
      if (active) setSettled(true);
    }
    poll();
    return () => {
      active = false;
    };
  }, [status, refreshAuth, refreshCart]);

  const latestStatus = orders[0]?.status;

  const view: View = useMemo(() => {
    if (status === 'fail') return 'failed';
    if (latestStatus === 'paid') return 'paid';
    if (latestStatus === 'cancelled') return 'failed';
    return settled ? 'failed' : 'pending';
  }, [status, latestStatus, settled]);

  if (view === 'pending') {
    return (
      <section className="sec">
        <div className="wrap result-page">
          <div className="eb">Оплата</div>
          <h1 className="d2">Проверяем платёж…</h1>
          <p>Подождите пару секунд — подтверждаем оплату у банка.</p>
        </div>
      </section>
    );
  }

  const isPaid = view === 'paid';

  return (
    <section className="sec">
      <div className="wrap result-page">
        <div className="eb">{isPaid ? 'Оплата прошла' : 'Оплата не завершена'}</div>
        <h1 className="d2">{isPaid ? 'Спасибо!' : 'Что-то пошло не так'}</h1>
        <p>
          {isPaid
            ? 'Заказ оплачен. Подписка или тренировка появится в личном кабинете.'
            : 'Платёж не был завершён, деньги не списаны. Попробуйте оформить заказ ещё раз.'}
        </p>
        <div className="hero-actions">
          {isPaid ? (
            <Link className="btn btn-primary" to="/me">
              Личный кабинет
            </Link>
          ) : (
            <Link className="btn btn-primary" to="/cart">
              Корзина
            </Link>
          )}
          <Link className="btn btn-ghost" to="/">
            На главную
          </Link>
        </div>
      </div>
    </section>
  );
}
