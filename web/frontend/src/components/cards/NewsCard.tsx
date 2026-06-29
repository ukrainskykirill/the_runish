import { formatDate } from '../../api/client';
import type { NewsItem } from '../../api/types';
import { CalendarIcon, PinIcon, StarBurstIcon } from '../icons';

const HEADER_ICONS = [CalendarIcon, StarBurstIcon, PinIcon];

interface NewsCardProps {
  news: NewsItem;
  iconIndex?: number;
  onReadMore?: (id: number) => void;
}

export function NewsCard({ news, iconIndex = 0, onReadMore }: NewsCardProps) {
  const HeaderIcon = HEADER_ICONS[iconIndex % HEADER_ICONS.length];
  const date = news.published_at ?? news.created_at;

  return (
    <article className="ncard">
      <div className="nh">
        <span className="nh-sl speedlines" />
        <HeaderIcon />
      </div>
      <div className="nb">
        <div className="date">{formatDate(date)}</div>
        <h3>{news.title}</h3>
        {onReadMore && (
          <button className="btn btn-ghost btn-sm ncard-more" onClick={() => onReadMore(news.id)}>
            Подробнее →
          </button>
        )}
      </div>
    </article>
  );
}
