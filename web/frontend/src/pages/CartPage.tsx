import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, ApiError, formatPrice } from '../api/client';
import { PhoneModal } from '../components/cart/PhoneModal';
import { CartIcon, CloseIcon } from '../components/icons';
import { useAuth } from '../context/AuthContext';
import { useCart } from '../context/CartContext';
import { useUI } from '../context/UIContext';

const CHECKOUT_ERRORS: Record<string, string> = {
  entry_fee_required: 'Сначала оплатите вступительный взнос — он открывает подписки.',
  already_owned: 'Эта позиция у вас уже есть.',
  multiple_subscriptions: 'В заказе может быть только одна подписка.',
  cart_empty: 'Корзина пуста.',
  cart_invalid: 'Товары в корзине недоступны — обновите корзину.',
  login_required: 'Войдите, чтобы оформить заказ.',
};

export function CartPage() {
  const { lines, total, remove } = useCart();
  const { user, refresh: refreshAuth } = useAuth();
  const { showToast } = useUI();
  const [checkingOut, setCheckingOut] = useState(false);
  const [showPhoneModal, setShowPhoneModal] = useState(false);

  const needPhone = !!user && !user.phone;

  async function doCheckout() {
    setCheckingOut(true);
    try {
      const { payment_url } = await api.checkout();
      window.location.href = payment_url;
    } catch (e) {
      setCheckingOut(false);
      if (e instanceof ApiError) {
        if (e.code === 'phone_required') {
          setShowPhoneModal(true);
          return;
        }
        showToast(CHECKOUT_ERRORS[e.code] ?? `Не удалось оформить заказ (${e.code})`);
      } else {
        showToast('Не удалось оформить заказ');
      }
    }
  }

  function handleCheckout() {
    if (lines.length === 0) return;
    if (needPhone) {
      setShowPhoneModal(true);
      return;
    }
    doCheckout();
  }

  async function handlePhoneSaved() {
    setShowPhoneModal(false);
    await refreshAuth();
    await doCheckout();
  }

  return (
    <>
      <section className="page-hero">
        <div className="hero-sl speedlines" />
        <div className="wrap">
          <div className="eb">Заказ</div>
          <h1 className="d2">Корзина</h1>
        </div>
      </section>
      <section className="sec">
        <div className="wrap">
          {lines.length === 0 ? (
            <div className="empty-state">
              <CartIcon />
              <div>Корзина пуста</div>
              <p style={{ marginTop: 16 }}>
                <Link className="btn btn-primary btn-sm" to="/runners">
                  Перейти к тренировкам
                </Link>
              </p>
            </div>
          ) : (
            <div className="cart-page-lines">
              {lines.map((line) => (
                <div className="cart-line" key={line.service_id}>
                  <div className="cl-t">
                    <div className="t">{line.title}</div>
                    <div className="m">
                      {line.qty > 1 ? `${line.qty} × ` : ''}
                      {formatPrice(line.price_kop)}
                    </div>
                  </div>
                  <button className="rm" aria-label="Удалить" onClick={() => remove(line.service_id)}>
                    <CloseIcon className="i i-sm" style={{ width: 14, height: 14 }} />
                  </button>
                </div>
              ))}
              <div className="cart-total">
                <span className="lab">Итого</span>
                <span className="v">{formatPrice(total)}</span>
              </div>

              <button
                className="btn btn-primary btn-block"
                onClick={handleCheckout}
                disabled={checkingOut}
              >
                Оформить и оплатить
              </button>
            </div>
          )}
        </div>
      </section>
      {showPhoneModal ? (
        <PhoneModal onSuccess={handlePhoneSaved} onClose={() => setShowPhoneModal(false)} />
      ) : null}
    </>
  );
}
