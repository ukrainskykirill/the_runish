const BRAND_ROW = (
  <span>
    The Runish <i className="star">✦</i> Рожденные двигаться <i className="star">✦</i> The Runish{' '}
    <i className="star">✦</i> Рожденные двигаться <i className="star">✦</i>
  </span>
);

function promoRow(left: number) {
  return (
    <span>
      🔥 Льготная цена на подписку для первых 30 <i className="star">✦</i> Осталось {left}{' '}
      {plural(left, 'место', 'места', 'мест')} <i className="star">✦</i> The Runish{' '}
      <i className="star">✦</i>
    </span>
  );
}

function plural(n: number, one: string, few: string, many: string) {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return one;
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few;
  return many;
}

interface MarqueeProps {
  promoActive?: boolean;
  left?: number;
}

export function Marquee({ promoActive = false, left = 0 }: MarqueeProps) {
  const row = promoActive && left > 0 ? promoRow(left) : BRAND_ROW;
  return (
    <div className="marquee" aria-hidden="true">
      <div className="marquee-row">
        {row}
        {row}
      </div>
    </div>
  );
}
