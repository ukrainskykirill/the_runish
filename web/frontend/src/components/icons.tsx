import type { SVGProps } from 'react';

type IconProps = SVGProps<SVGSVGElement>;

function base(props: IconProps, children: React.ReactNode) {
  const { className = 'i', ...rest } = props;
  return (
    <svg
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.9}
      strokeLinecap="round"
      strokeLinejoin="round"
      {...rest}
    >
      {children}
    </svg>
  );
}

export function CartIcon(props: IconProps) {
  return base(props, (
    <>
      <path d="M5 7h14l-1.2 9.4a2 2 0 0 1-2 1.6H8.2a2 2 0 0 1-2-1.6z" />
      <path d="M9 7V5.5a3 3 0 0 1 6 0V7" />
    </>
  ));
}

export function TelegramIcon(props: IconProps) {
  const { className = 'i', ...rest } = props;
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor" stroke="none" {...rest}>
      <path d="M21.5 4.3 2.9 11.5c-.9.4-.9 1.6 0 1.9l4.6 1.5 1.8 5.6c.2.7 1.1.9 1.6.3l2.5-2.7 4.7 3.5c.6.4 1.5.1 1.7-.7l3.4-15.4c.2-.9-.7-1.7-1.7-1.3z" />
    </svg>
  );
}

export function ChevronDownIcon(props: IconProps) {
  return base(props, <path d="m6 9 6 6 6-6" />);
}

export function ChevronRightIcon(props: IconProps) {
  return base(props, <path d="m9 6 6 6-6 6" />);
}

export function ArrowUpRightIcon(props: IconProps) {
  return base(props, (
    <>
      <path d="M7 17 17 7" />
      <path d="M9 7h8v8" />
    </>
  ));
}

export function CheckIcon(props: IconProps) {
  return base(props, <path d="M20 6 9 17l-5-5" />);
}

export function MenuIcon(props: IconProps) {
  return base(props, (
    <>
      <path d="M4 6h16" />
      <path d="M4 12h16" />
      <path d="M4 18h16" />
    </>
  ));
}

export function CloseIcon(props: IconProps) {
  return base(props, (
    <>
      <path d="M6 6l12 12" />
      <path d="M18 6 6 18" />
    </>
  ));
}

export function LogoutIcon(props: IconProps) {
  return base(props, (
    <>
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <polyline points="16 17 21 12 16 7" />
      <line x1="21" y1="12" x2="9" y2="12" />
    </>
  ));
}

export function ListIcon(props: IconProps) {
  return base(props, <path d="M4 7h16M4 12h16M4 17h10" />);
}

export function CalendarIcon(props: IconProps) {
  return base(props, (
    <>
      <rect x="3" y="6" width="18" height="13" rx="2.5" />
      <path d="M3 10h18M8 3v4M16 3v4" />
    </>
  ));
}

export function FlagIcon(props: IconProps) {
  return base(props, <path d="M5 4h14v16l-3-2-2 2-2-2-2 2-2-2-3 2z" />);
}

export function SupportIcon(props: IconProps) {
  return base(props, (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M9.5 9.5a2.5 2.5 0 1 1 3.4 2.3c-.6.3-.9.8-.9 1.4" />
      <circle cx="12" cy="16.5" r=".5" fill="currentColor" />
    </>
  ));
}

export function InfoIcon(props: IconProps) {
  return base(props, (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 8v.5M12 11v5" />
    </>
  ));
}

export function ClockIcon(props: IconProps) {
  return base(props, (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 8v4l3 2" />
    </>
  ));
}

export function BoltIcon(props: IconProps) {
  return base(props, <path d="M13 3 4 14h7l-1 7 9-11h-7z" />);
}

export function CommunityIcon(props: IconProps) {
  return base(props, (
    <>
      <circle cx="9" cy="8" r="3" />
      <circle cx="17" cy="9.5" r="2.4" />
      <path d="M3.5 19a5.5 5.5 0 0 1 11 0M15 19a4.5 4.5 0 0 1 6-3.9" />
    </>
  ));
}

export function BoxIcon(props: IconProps) {
  return base(props, (
    <>
      <rect x="3" y="6" width="18" height="13" rx="2.5" />
      <path d="M3 10h18" />
    </>
  ));
}

export function StarBurstIcon(props: IconProps) {
  return base(props, <path d="m12 3 2.3 5.6L20 9l-4.3 3.9L17 19l-5-3-5 3 1.3-6.1L4 9l5.7-.4z" />);
}

export function UserIcon(props: IconProps) {
  return base(props, (
    <>
      <circle cx="12" cy="8" r="3.5" />
      <path d="M4.5 20a7.5 7.5 0 0 1 15 0" />
    </>
  ));
}

export function PinIcon(props: IconProps) {
  return base(props, (
    <>
      <path d="M12 21s-7-5.7-7-11a7 7 0 0 1 14 0c0 5.3-7 11-7 11z" />
      <circle cx="12" cy="10" r="2.4" />
    </>
  ));
}
