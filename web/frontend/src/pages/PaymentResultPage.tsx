import { useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { api } from '../api/client';
import { useAuth } from '../context/AuthContext';
import { useCart } from '../context/CartContext';

interface PaymentResultPageProps {
  status: 'success' | 'fail';
}

type View = 'pending' | 'paid' | 'failed';

export function PaymentResultPage({ status }: PaymentResultPageProps) {
  const [searchParams] = useSearchParams();
  const order = searchParams.get('order');
  const { user, orders, refresh: refreshAuth } = useAuth();
  const { refresh: refreshCart } = useCart();

  // Реальный статус платежа из БД (его проставляет вебхук), доступный без сессии по order id.
  // Нужен для оплаты из mini app: редирект T-Bank открывается во внешнем браузере без cookie,
  // поэтому /api/me там пустой и на него полагаться нельзя.
  const [realStatus, setRealStatus] = useState<'paid' | 'pending' | 'failed' | 'refunded' | null>(null);
  const [settled, setSettled] = useState(false);

  useEffect(() => {
    let active = true;

    async function poll() {
      for (let i = 0; i < 10 && active; i++) {
        if (order) {
          try {
            const { status: s } = await api.paymentStatus(order);
            if (!active) return;
            setRealStatus(s);
            if (s === 'paid' || s === 'failed' || s === 'refunded') break;
          } catch {
            // заказ ещё не виден / сеть — повторим на следующем тике
          }
        }
        // В браузере с сессией параллельно обновим ЛК и очистим корзину;
        // во внешнем браузере (mini app) это безвредно вернёт пустые данные.
        await Promise.all([refreshAuth(), refreshCart()]);
        if (i < 9) await new Promise((resolve) => window.setTimeout(resolve, 1500));
      }
      if (active) setSettled(true);
    }

    poll();
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [order]);

  const latestOrderStatus = orders[0]?.status;

  const view: View = useMemo(() => {
    // Источник правды — реальный статус платежа, если order id известен.
    if (realStatus === 'paid') return 'paid';
    if (realStatus === 'failed' || realStatus === 'refunded') return 'failed';
    if (!order) {
      // Старый путь (без order id): опираемся на маршрут и /api/me.
      if (status === 'fail') return 'failed';
      if (latestOrderStatus === 'paid') return 'paid';
      if (latestOrderStatus === 'cancelled') return 'failed';
    }
    return settled ? 'failed' : 'pending';
  }, [realStatus, order, status, latestOrderStatus, settled]);

  // Оплата из mini app: пользователь во внешнем браузере, сессии нет — предлагаем вернуться в бота.
  const outsideSession = !!order && !user;

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
            ? outsideSession
              ? 'Оплата подтверждена. Вернитесь в Telegram — подписка уже активна.'
              : 'Заказ оплачен. Подписка или тренировка появится в личном кабинете.'
            : 'Платёж не был завершён, деньги не списаны. Попробуйте оформить заказ ещё раз.'}
        </p>
        {outsideSession ? null : (
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
        )}
      </div>
    </section>
  );
}
