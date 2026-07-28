interface SubscribeBannerProps {
  price: string;
  onBuy: () => void;
}

export function SubscribeBanner({ price, onBuy }: SubscribeBannerProps) {
  if (!price) return null;
  return (
    <section className="sb-band">
      <div className="wrap">
        <div className="subscribe-banner">
          <div className="sb-lines speedlines" />
          <div className="sb-copy">
            <div className="sb-kicker">The Runish Community</div>
            <div className="sb-title">Все тренировки в одной подписке</div>
            <div className="sb-note">Клубные тренировки и общий тренировочный план</div>
          </div>
          <div className="sb-buy">
            <div className="sb-price">
              {price}
              <span>/мес</span>
            </div>
            <button type="button" className="sb-cta" onClick={onBuy}>
              Купить
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}
