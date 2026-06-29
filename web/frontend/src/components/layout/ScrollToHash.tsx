import { useEffect } from 'react';
import { useLocation } from 'react-router-dom';

// ScrollToHash скроллит к секции с id из hash после смены роута.
// Нужно для SPA: при переходе на "/#schedule" с другой страницы секция
// появляется только после рендера HomePage, поэтому нативный скролл браузера
// не срабатывает. Делаем это сами, с ретраями на случай асинхронного контента.
export function ScrollToHash() {
  const { pathname, hash } = useLocation();

  useEffect(() => {
    if (!hash) {
      window.scrollTo({ top: 0, left: 0, behavior: 'auto' });
      return;
    }
    const id = hash.slice(1);
    let tries = 0;
    let timer: number;

    const scroll = () => {
      const el = document.getElementById(id);
      if (el) {
        el.scrollIntoView({ behavior: 'smooth' });
      } else if (tries++ < 20) {
        timer = window.setTimeout(scroll, 50);
      }
    };

    timer = window.setTimeout(scroll, 0);
    return () => window.clearTimeout(timer);
  }, [pathname, hash]);

  return null;
}
