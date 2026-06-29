import type { ReactNode } from 'react';

interface FeatureCardProps {
  icon: ReactNode;
  title: string;
  children: ReactNode;
}

export function FeatureCard({ icon, title, children }: FeatureCardProps) {
  return (
    <div className="fcard">
      <div className="fi">{icon}</div>
      <h3>{title}</h3>
      <p>{children}</p>
    </div>
  );
}
