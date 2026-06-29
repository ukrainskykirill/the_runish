import { formatPrice } from '../../api/client';
import type { MerchItem } from '../../api/types';
import monogramCream from '../../assets/monogram-cream.png';

interface MerchCardProps {
  item: MerchItem;
}

export function MerchCard({ item }: MerchCardProps) {
  return (
    <div className="mcard">
      <div className="mh">
        <span className="mh-sl speedlines" />
        <img src={monogramCream} alt={item.title} />
      </div>
      <div className="mb">
        <div>
          <h3>{item.title}</h3>
          <div className="desc">{item.description}</div>
        </div>
        <div className="price">{formatPrice(item.price_kop)}</div>
      </div>
    </div>
  );
}
