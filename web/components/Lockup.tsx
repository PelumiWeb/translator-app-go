import { WORDMARK } from "@/lib/site";
import s from "./Lockup.module.css";

// The mark, from docs/design/pearl/assets/shinar-mark-on-dark.svg: two voices
// (mint is them, warm white is you), their overlap, and the plain beneath.
export function Mark({ width = 40 }: { width?: number }) {
  return (
    <svg viewBox="6 16 108 50" width={width} height={(width * 50) / 108} aria-hidden="true" focusable="false">
      <line x1="8" y1="62" x2="112" y2="62" stroke="#F3EEE4" strokeOpacity="0.5" strokeWidth="2.5" strokeLinecap="round" />
      <circle cx="46" cy="40" r="22" fill="#7FD1C1" />
      <circle cx="74" cy="40" r="22" fill="#F3EEE4" />
      <path d="M60 23.03A22 22 0 0 1 60 56.97A22 22 0 0 1 60 23.03Z" fill="#B9E0D3" />
    </svg>
  );
}

// The horizontal lockup: the mark at 40 wide, the wordmark at 20/500, -0.04em.
export function Lockup({ size = "regular" }: { size?: "regular" | "small" }) {
  const markWidth = size === "small" ? 32 : 40;
  return (
    <span className={`${s.lockup} ${s[size]}`}>
      <Mark width={markWidth} />
      <span className={s.word}>{WORDMARK}</span>
    </span>
  );
}
