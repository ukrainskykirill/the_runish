interface PromoCounterProps {
  paid: number;
  limit: number;
  active: boolean;
}

export function PromoCounter({ paid, limit, active }: PromoCounterProps) {
  if (!active || paid >= limit) return null;

  const left = limit - paid;
  const pct = Math.min(100, Math.round((paid / limit) * 100));

  return (
    <div className="promo-counter">
      <div className="pc-head">
        <span className="pc-badge">🔥 Акция</span>
        <span className="pc-sub">Льготная цена на подписку для первых {limit}</span>
      </div>
      <div className="pc-count">
        Осталось <b>{left}</b> из {limit} мест
      </div>
      <div className="pc-bar">
        <div className="pc-bar-fill" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
