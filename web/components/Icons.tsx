// The few glyphs the page draws, after the outline style the design uses
// (Lucide-like, stroke 1.75). Filled glyphs for play and pause, as in the app.

type IconProps = { size?: number };

export function PlayGlyph({ size = 15 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M7 4.6v14.8a1 1 0 0 0 1.52.86l12.2-7.4a1 1 0 0 0 0-1.72L8.52 3.74A1 1 0 0 0 7 4.6Z" fill="currentColor" />
    </svg>
  );
}

export function PauseGlyph({ size = 15 }: IconProps) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect x="5" y="3.5" width="5" height="17" rx="1.2" fill="currentColor" />
      <rect x="14" y="3.5" width="5" height="17" rx="1.2" fill="currentColor" />
    </svg>
  );
}

export function SpeakerIcon({ size = 28 }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <path d="M11 5 6 9H3v6h3l5 4V5Z" />
      <path d="M15.5 8.5a5 5 0 0 1 0 7" />
      <path d="M18.8 5.2a9.5 9.5 0 0 1 0 13.6" />
    </svg>
  );
}
