import { useEffect, useRef } from 'react';

// HeroVideo — hero-петля с отказоустойчивой загрузкой.
// Постер показывается мгновенно; как только видео готово — стартует автоматически.
// Если автоплей заблокирован браузером или поток сорвался — повторяем попытку,
// а при полном фейле остаётся постер (без чёрного экрана).
export function HeroVideo() {
  const ref = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const v = ref.current;
    if (!v) return;

    let retries = 0;
    const tryPlay = () => {
      v.play().catch(() => {
        // автоплей может быть заблокирован/сорван — повторим несколько раз
        if (retries++ < 5) {
          setTimeout(tryPlay, 600);
        }
      });
    };

    const onReady = () => tryPlay();
    const onStalled = () => {
      // поток подвис — пробуем перезагрузить и снова запустить
      try {
        v.load();
      } catch {
        /* ignore */
      }
    };

    v.addEventListener('loadeddata', onReady);
    v.addEventListener('canplay', onReady);
    v.addEventListener('stalled', onStalled);
    v.addEventListener('error', onStalled);

    // первая попытка (если видео уже в кэше)
    tryPlay();

    // если за 4с воспроизведение так и не пошло — мягкий перезапуск загрузки
    const kick = window.setTimeout(() => {
      if (v.paused || v.readyState < 2) {
        onStalled();
      }
    }, 4000);

    return () => {
      v.removeEventListener('loadeddata', onReady);
      v.removeEventListener('canplay', onReady);
      v.removeEventListener('stalled', onStalled);
      v.removeEventListener('error', onStalled);
      window.clearTimeout(kick);
    };
  }, []);

  return (
    <video
      ref={ref}
      src="/hero-run.mp4"
      poster="/hero-run-poster.jpg"
      preload="auto"
      autoPlay
      muted
      loop
      playsInline
    />
  );
}
