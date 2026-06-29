import { ArrowUpRightIcon, TelegramIcon } from '../icons';

interface TelegramCardProps {
  href: string;
  title: string;
  subtitle: string;
}

export function TelegramCard({ href, title, subtitle }: TelegramCardProps) {
  return (
    <a className="tgcard" href={href} target="_blank" rel="noopener noreferrer">
      <span className="tg-sl speedlines" />
      <span className="tgi">
        <TelegramIcon />
      </span>
      <span className="tgt">
        <span className="t">{title}</span>
        <span className="s">{subtitle}</span>
      </span>
      <span className="arr">
        <ArrowUpRightIcon />
      </span>
    </a>
  );
}
