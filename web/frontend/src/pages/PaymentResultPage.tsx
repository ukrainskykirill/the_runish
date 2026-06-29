import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';
import { useCart } from '../context/CartContext';

interface PaymentResultPageProps {
  status: 'success' | 'fail';
}

export function PaymentResultPage({ status }: PaymentResultPageProps) {
  const { refresh: refreshAuth } = useAuth();
  const { refresh: refreshCart } = useCart();

  useEffect(() => {
    let active = true;
    async function refreshAfterPayment() {
      for (let i = 0; i < 4 && active; i++) {
        await Promise.all([refreshAuth(), refreshCart()]);
        if (i < 3) {
          await new Promise((resolve) => window.setTimeout(resolve, 1500));
        }
      }
    }
    refreshAfterPayment();
    return () => {
      active = false;
    };
  }, [refreshAuth, refreshCart]);

  const isSuccess = status === 'success';

  return (
    <section className="sec">
      <div className="wrap result-page">
        <div className="eb">{isSuccess ? 'Оплата прошла' : 'Оплата не выполнена'}</div>
        <h1 className="d2">{isSuccess ? 'Спасибо!' : 'Что-то пошло не так'}</h1>
        <p>
          {isSuccess
            ? 'Заказ оплачен. Подписка или тренировка появится в личном кабинете.'
            : 'Платёж не был завершён. Попробуйте оформить заказ ещё раз.'}
        </p>
        <div className="hero-actions">
          {isSuccess ? (
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
