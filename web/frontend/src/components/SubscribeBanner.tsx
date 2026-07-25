interface SubscribeBannerProps {
  price: string;
  onBuy: () => void;
}

export function SubscribeBanner({ price, onBuy }: SubscribeBannerProps) {
  if (!price) return null;
  return (
    <div className="subscribe-banner">
      <button type="button" className="sb-cta" onClick={onBuy}>
        Купить за {price}/мес
      </button>
    </div>
  );
}
