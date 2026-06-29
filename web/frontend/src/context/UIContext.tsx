import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from 'react';

interface LoginModalState {
  open: boolean;
  reason: string | null;
  pendingServiceId: number | null;
}

interface UIContextValue {
  toastMessage: string | null;
  toastVisible: boolean;
  showToast: (message: string) => void;
  loginModal: LoginModalState;
  openLoginModal: (opts?: { reason?: string; pendingServiceId?: number }) => void;
  closeLoginModal: () => void;
}

const UIContext = createContext<UIContextValue | null>(null);

export function UIProvider({ children }: { children: ReactNode }) {
  const [toastMessage, setToastMessage] = useState<string | null>(null);
  const [toastVisible, setToastVisible] = useState(false);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [loginModal, setLoginModal] = useState<LoginModalState>({
    open: false,
    reason: null,
    pendingServiceId: null,
  });

  const showToast = useCallback((message: string) => {
    setToastMessage(message);
    setToastVisible(true);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToastVisible(false), 2200);
  }, []);

  const openLoginModal = useCallback((opts?: { reason?: string; pendingServiceId?: number }) => {
    setLoginModal({
      open: true,
      reason: opts?.reason ?? null,
      pendingServiceId: opts?.pendingServiceId ?? null,
    });
  }, []);

  const closeLoginModal = useCallback(() => {
    setLoginModal({ open: false, reason: null, pendingServiceId: null });
  }, []);

  return (
    <UIContext.Provider
      value={{ toastMessage, toastVisible, showToast, loginModal, openLoginModal, closeLoginModal }}
    >
      {children}
    </UIContext.Provider>
  );
}

export function useUI(): UIContextValue {
  const ctx = useContext(UIContext);
  if (!ctx) throw new Error('useUI must be used within UIProvider');
  return ctx;
}
