const ROW = (
  <span>
    The Runish <i className="star">✦</i> Живи в движении <i className="star">✦</i> Бегаем вместе{' '}
    <i className="star">✦</i> The Runish <i className="star">✦</i> Живи в движении{' '}
    <i className="star">✦</i> Бегаем вместе <i className="star">✦</i>
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
