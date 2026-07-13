import { useState } from 'react';
import { api, ApiError, formatPrice } from '../../api/client';
import type { Service } from '../../api/types';
import { useAuth } from '../../context/AuthContext';
import { useCart } from '../../context/CartContext';
import { useUI } from '../../context/UIContext';
import { useAsync } from '../../lib/useAsync';
import { useTelegram } from '../../hooks/useTelegram';
import { ErrorState, LoadingGrid } from '../../components/LoadState';
import { PlusIcon } from '../../components/icons';

const CHECKOUT_ERRORS: Record<string, string> = {
  entry_fee_required: 'Сначала оплатите вступительный взнос — он открывает подписки.',
  already_owned: 'Эта позиция у вас уже есть.',
  multiple_subscriptions: 'В заказе может быть только одна подписка.',
  cart_empty: 'Корзина пуста.',
  cart_invalid: 'Товары в корзине недоступны — обновите корзину.',
  login_required: 'Войдите, чтобы оформить заказ.',
  phone_required: 'Поделитесь номером телефона в профиле, чтобы оплатить заказ.',
};

function PlanCard({ service, onAdd }: { service: Service; onAdd: (id: number) => void }) {
  const price = service.effective_price_kop;
  const isFree = service.kind === 'free' || price === 0;
  const featured = service.kind === 'subscription' || service.kind === 'bundle';
  const disabled = service.locked || service.owned;

  let btnLabel = isFree ? 'Записаться' : 'В корзину';
  if (service.owned) {
    btnLabel = service.kind === 'subscription' ? 'Активна' : 'Оплачено';
  } else if (service.locked) {
    btnLabel = 'Сначала вступление';
  }

  return (
    <div className={'plan' + (featured ? ' feat' : '')}>
      {featured ? <div className="psl speedlines" /> : null}
      <div className="prow">
        <span className="chip" style={featured ? { background: 'var(--cream-light)', color: 'var(--runish-red-deep)', borderColor: 'transparent' } : undefined}>
          {isFree ? 'Бесплатно' : service.kind === 'subscription' ? 'Подписка' : 'Разовая'}
        </span>
      </div>
      <h3>{service.title}</h3>
      <div className="desc">{service.description}</div>
      <div className="pb">
        <span className={'price' + (isFree ? ' free' : '')}>{formatPrice(price)}</span>
        <button
          className="pill-btn"
          style={featured ? { background: 'var(--cream-light)', color: 'var(--runish-red-deep)', borderColor: 'transparent' } : undefined}
          disabled={disabled}
          onClick={() => onAdd(service.id)}
        >
          <PlusIcon className="i i-xs" />
          {btnLabel}
        </button>
      </div>
    </div>
  );
}

export function MiniSubscriptionsPage() {
  const tg = useTelegram();
  const { add, lines, total } = useCart();
  const { refresh: refreshAuth } = useAuth();
  const { showToast } = useUI();
  const { data, loading, error } = useAsync(() => api.catalog(), []);
  const services = data?.services ?? [];
  const [checkingOut, setCheckingOut] = useState(false);
  const [waitingPayment, setWaitingPayment] = useState(false);

  async function handleCheckout() {
    if (lines.length === 0 || checkingOut) return;
    setCheckingOut(true);
    try {
      const { payment_url } = await api.checkout();
      if (tg) tg.openLink(payment_url);
      else window.open(payment_url, '_blank', 'noopener,noreferrer');

      setWaitingPayment(true);
      for (let i = 0; i < 20; i++) {
        await new Promise((resolve) => window.setTimeout(resolve, 3000));
        const me = await api.me();
        const latest = me.orders[0];
        if (latest?.status === 'paid' || latest?.status === 'cancelled') break;
      }
      await refreshAuth();
    } catch (e) {
      const code = e instanceof ApiError ? e.code : 'unknown_error';
      showToast(CHECKOUT_ERRORS[code] ?? 'Не удалось оформить заказ');
    } finally {
      setCheckingOut(false);
      setWaitingPayment(false);
    }
  }

  return (
    <div className="pad stack" style={{ paddingBottom: lines.length ? 96 : 16 }}>
      {loading ? (
        <LoadingGrid />
      ) : error ? (
        <ErrorState message="Не удалось загрузить каталог" />
      ) : (
        services.map((s) => <PlanCard key={s.id} service={s} onAdd={add} />)
      )}

      {lines.length > 0 ? (
        <div className="mainbtn">
          <button onClick={handleCheckout} disabled={checkingOut}>
            {waitingPayment ? 'Ожидаем оплату…' : checkingOut ? 'Оформляем…' : `В корзину · ${formatPrice(total)}`}
          </button>
        </div>
      ) : null}
    </div>
  );
}
