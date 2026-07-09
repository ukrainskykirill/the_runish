const ROW = (
  <span>
    The Runish <i className="star">✦</i> Рожденные двигаться <i className="star">✦</i> The Runish{' '}
    <i className="star">✦</i> Рожденные двигаться <i className="star">✦</i>
  </span>
);

export function Marquee() {
  return (
    <div className="marquee" aria-hidden="true">
      <div className="marquee-row">
        {ROW}
        {ROW}
      </div>
    </div>
  );
}
