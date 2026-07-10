import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, ApiError, formatPrice } from '../../api/client';
import { useAuth } from '../../context/AuthContext';
import { useCart } from '../../context/CartContext';
import { useUI } from '../../context/UIContext';
import { CartIcon, CloseIcon } from '../icons';
import { PhoneModal } from './PhoneModal';

function plural(n: number): string {
  const a = n % 10;
  const b = n % 100;
  if (a === 1 && b !== 11) return 'позиция';
  if (a >= 2 && a <= 4 && (b < 10 || b >= 20)) return 'позиции';
  return 'позиций';
}

const CHECKOUT_ERRORS: Record<string, string> = {
  entry_fee_required: 'Сначала оплатите вступительный взнос — он открывает подписки.',
  already_owned: 'Эта позиция у вас уже есть.',
  multiple_subscriptions: 'В заказе может быть только одна подписка.',
  cart_empty: 'Корзина пуста.',
  cart_invalid: 'Товары в корзине недоступны — обновите корзину.',
  login_required: 'Войдите, чтобы оформить заказ.',
};

interface CartDropdownProps {
  open: boolean;
  onClose: () => void;
}

export function CartDropdown({ open, onClose }: CartDropdownProps) {
  const { lines, total, count, remove } = useCart();
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
    <div className={`dd ${open ? 'open' : ''}`}>
      <div className="dd-head">
        <h4>Корзина</h4>
        <span className="chip chip-red">
          {count} {plural(count)}
        </span>
      </div>
      <div className="dd-body">
        {lines.length === 0 ? (
          <div className="cart-empty">
            <CartIcon />
            <div>Корзина пуста</div>
          </div>
        ) : (
          lines.map((line) => (
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
          ))
        )}
      </div>
      {lines.length > 0 ? (
        <div className="dd-foot">
          <div className="cart-total">
            <span className="lab">Итого</span>
            <span className="v">{formatPrice(total)}</span>
          </div>
          <button className="btn btn-primary" onClick={handleCheckout} disabled={checkingOut}>
            Оформить и оплатить
          </button>
          <Link className="view" to="/runners" onClick={onClose}>
            Перейти к пробежкам
          </Link>
        </div>
      ) : null}
      {showPhoneModal ? (
        <PhoneModal onSuccess={handlePhoneSaved} onClose={() => setShowPhoneModal(false)} />
      ) : null}
    </div>
  );
}
