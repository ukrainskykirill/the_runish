import { useState } from 'react';
import { Link } from 'react-router-dom';
import { api, ApiError, formatPrice } from '../api/client';
import { CartIcon, CloseIcon } from '../components/icons';
import { useAuth } from '../context/AuthContext';
import { useCart } from '../context/CartContext';
import { useUI } from '../context/UIContext';

// Человекочитаемые сообщения под коды ошибок checkout (кроме phone_required —
// он ведёт в бота). Неизвестный код покажем как есть, чтобы было видно причину.
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
  const { user, botUsername, refresh: refreshAuth } = useAuth();
  const { showToast } = useUI();
  const [checkingOut, setCheckingOut] = useState(false);

  // Телефон обязателен для чека: если у юзера его нет — оплату не показываем.
  const needPhone = !!user && !user.phone;

  // Диплинк к боту за телефоном: ?start=phone → бот сразу просит поделиться номером.
  const phoneLink = botUsername ? `https://t.me/${botUsername}?start=phone` : '#';

  // Перебрасываем в бота за номером. Используем синхронную навигацию (не window.open):
  // попап после await браузер блокирует, а переход по location — нет.
  function goToBotForPhone() {
    if (!botUsername) {
      showToast('Бот временно недоступен — попробуйте позже');
      return;
    }
    showToast('Для оплаты нужен номер телефона — открываем бота…');
    window.location.href = phoneLink;
  }

  async function handleCheckout() {
    if (lines.length === 0) return;
    // Нет телефона — сразу в бота за номером, без попытки оплаты.
    if (needPhone) {
      goToBotForPhone();
      return;
    }
    setCheckingOut(true);
    try {
      const { payment_url } = await api.checkout();
      window.location.href = payment_url;
    } catch (e) {
      setCheckingOut(false);
      if (e instanceof ApiError) {
        if (e.code === 'phone_required') {
          // Бэк сказал, что телефона нет — ведём в бота (без await, чтобы переход сработал).
          goToBotForPhone();
          return;
        }
        showToast(CHECKOUT_ERRORS[e.code] ?? `Не удалось оформить заказ (${e.code})`);
      } else {
        showToast('Не удалось оформить заказ');
      }
    }
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
                  Перейти к пробежкам
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

              {needPhone ? (
                <div className="phone-gate">
                  <div className="phone-gate-t">
                    Чтобы оплатить, нужен телефон — он попадёт в чек. Откройте бота в Telegram
                    и поделитесь номером (или отправьте команду <b>/phone</b>).
                  </div>
                  <a
                    className="btn btn-primary btn-block"
                    href={phoneLink}
                    target="_blank"
                    rel="noreferrer"
                  >
                    Поделиться номером в Telegram
                  </a>
                  <button className="btn btn-ghost btn-sm btn-block" onClick={() => refreshAuth()}>
                    Я поделился — обновить
                  </button>
                </div>
              ) : (
                <button
                  className="btn btn-primary btn-block"
                  onClick={handleCheckout}
                  disabled={checkingOut}
                >
                  Оформить и оплатить
                </button>
              )}
            </div>
          )}
        </div>
      </section>
    </>
  );
}
