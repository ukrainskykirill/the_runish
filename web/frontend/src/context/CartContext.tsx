import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import { api, ApiError } from '../api/client';
import type { CartLine } from '../api/types';
import { useUI } from './UIContext';

interface CartContextValue {
  lines: CartLine[];
  total: number;
  count: number;
  loading: boolean;
  add: (serviceId: number) => Promise<void>;
  remove: (serviceId: number) => Promise<void>;
  refresh: () => Promise<void>;
}

const CartContext = createContext<CartContextValue | null>(null);

export function CartProvider({ children }: { children: ReactNode }) {
  const [lines, setLines] = useState<CartLine[]>([]);
  const [total, setTotal] = useState(0);
  const [count, setCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const { showToast, openLoginModal } = useUI();

  const refresh = useCallback(async () => {
    try {
      const data = await api.cart();
      setLines(data.lines);
      setTotal(data.total);
      setCount(data.count);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const add = useCallback(async (serviceId: number) => {
    try {
      const data = await api.cartAdd(serviceId);
      setLines(data.lines);
      setTotal(data.total);
      setCount(data.count);
      showToast('Добавлено в корзину');
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) {
        openLoginModal({
          reason: 'Войди через Telegram, чтобы добавить в корзину и оплатить.',
          pendingServiceId: serviceId,
        });
      } else if (e instanceof ApiError && e.code === 'subscription_already_in_cart') {
        showToast('В корзине уже есть подписка — оформите её отдельно');
      } else {
        showToast('Не удалось добавить в корзину');
      }
    }
  }, [showToast, openLoginModal]);

  const remove = useCallback(async (serviceId: number) => {
    const data = await api.cartRemove(serviceId);
    setLines(data.lines);
    setTotal(data.total);
    setCount(data.count);
  }, []);

  return (
    <CartContext.Provider value={{ lines, total, count, loading, add, remove, refresh }}>
      {children}
    </CartContext.Provider>
  );
}

export function useCart(): CartContextValue {
  const ctx = useContext(CartContext);
  if (!ctx) throw new Error('useCart must be used within CartProvider');
  return ctx;
}
