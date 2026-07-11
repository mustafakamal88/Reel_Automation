import { useId } from 'react';

interface LogoMarkProps {
  className?: string;
  title?: string;
}

export function LogoMark({ className = 'logo-mark', title = 'TrendCortex' }: LogoMarkProps) {
  const maskId = `trendcortex-logo-${useId().replace(/:/g, '')}`;

  return (
    <svg className={className} viewBox="0 0 32 32" role="img" aria-label={title} focusable="false">
      <mask id={maskId} maskUnits="userSpaceOnUse">
        <rect width="32" height="32" fill="white" />
        <circle cx="16" cy="7.5" r="2.05" fill="black" />
        <circle cx="22.8" cy="11.2" r="2.05" fill="black" />
        <circle cx="22.8" cy="20.8" r="2.05" fill="black" />
        <circle cx="16" cy="24.5" r="2.05" fill="black" />
        <circle cx="9.2" cy="20.8" r="2.05" fill="black" />
        <circle cx="9.2" cy="11.2" r="2.05" fill="black" />
        <path d="M9.6 25.4L25.4 9.6" stroke="black" strokeWidth="3.4" strokeLinecap="round" />
      </mask>
      <path
        className="logo-reel"
        mask={`url(#${maskId})`}
        d="M16 3C8.82 3 3 8.82 3 16s5.82 13 13 13 13-5.82 13-13S23.18 3 16 3Zm0 7.85A5.15 5.15 0 1 0 16 21.15 5.15 5.15 0 0 0 16 10.85Z"
      />
    </svg>
  );
}
