import { formatPrice } from '../../api/client';
import type { Service } from '../../api/types';
import { ClockIcon } from '../icons';

interface ServiceCardProps {
  service: Service;
  onAdd: (id: number) => void;
}

const CHIP_LABEL: Record<string, string> = {
  free: 'Бесплатно',
  entry: 'Вступление',
  bundle: 'Старт в клуб',
  subscription: 'Подписка',
  training: 'Услуга',
};

export function ServiceCard({ service, onAdd }: ServiceCardProps) {
  const price = service.effective_price_kop;
  const isFree = service.kind === 'free' || price === 0;
  const discounted = !isFree && price < service.price_kop;
  const pct = discounted ? Math.round((1 - price / service.price_kop) * 100) : 0;

  const chipLabel = CHIP_LABEL[service.kind] ?? 'Услуга';
  const chipClass = isFree ? 'chip chip-track' : 'chip';

  const disabled = service.locked || service.owned;
  let btnLabel = isFree ? 'Записаться' : 'В корзину';
  if (service.owned) {
    btnLabel = service.kind === 'subscription' ? 'Подписка активна' : 'Вступление оплачено';
  } else if (service.locked) {
    btnLabel = 'Сначала вступление';
  }

  return (
    <div className="ccard">
      <div className="ct-top">
        <span className={chipClass}>{chipLabel}</span>
        {discounted ? <span className="chip chip-red">−{pct}%</span> : null}
      </div>
      <h3>{service.title}</h3>
      <p className="desc">{service.description}</p>
      {service.duration_days ? (
        <div className="dur">
          <ClockIcon className="i i-sm" />
          {service.duration_days} дней
        </div>
      ) : null}
      <div className="price">
        {discounted ? <span className="old">{formatPrice(service.price_kop)}</span> : null}
        <span className={`v${isFree ? ' free' : ''}`}>{formatPrice(price)}</span>
      </div>
      <button className="btn btn-ghost" onClick={() => onAdd(service.id)} disabled={disabled}>
        {btnLabel}
      </button>
      {service.locked ? (
        <p className="ccard-hint">Доступно после оплаты вступления в клуб</p>
      ) : null}
    </div>
  );
}
