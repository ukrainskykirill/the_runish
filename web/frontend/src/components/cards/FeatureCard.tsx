import type { ReactNode } from 'react';

interface FeatureCardProps {
  icon: ReactNode;
  title: string;
  children: ReactNode;
  action?: ReactNode;
}

export function FeatureCard({ icon, title, children, action }: FeatureCardProps) {
  return (
    <div className="fcard">
      <div className="fi">{icon}</div>
      <h3>{title}</h3>
      <p>{children}</p>
      {action ? <div className="fcard-action">{action}</div> : null}
    </div>
  );
}
