import type { CSSProperties, ReactNode } from "react";
import { PlayGlyph } from "./Icons";
import s from "./NoteParts.module.css";

// The parts of a voice note card, as the handoff draws them. Animations that
// belong to one scene (a waveform filling, a bar growing) are passed in as a
// class from that scene's own stylesheet.

// 22 dots, 3 to 10 wide by loudness. A fixed shape: there is no real audio.
const LEVELS = [4, 6, 3, 8, 10, 7, 5, 9, 6, 4, 8, 10, 6, 3, 7, 9, 5, 4, 8, 6, 3, 5];

type DotsProps = {
  tone: "them" | "onYou";
  // Draws the played dots too, over the unplayed ones, revealed by this class
  // (a clip-path animation). Without it every dot is unplayed.
  playedClassName?: string;
};

export function Dots({ tone, playedClassName }: DotsProps) {
  const row = (className: string) => (
    <span className={`${s.dotRow} ${className}`}>
      {LEVELS.map((d, i) => (
        <span key={i} className={s.dot} style={{ width: d, height: d }} />
      ))}
    </span>
  );
  return (
    <span className={`${s.dots} ${s[tone]}`}>
      {row(s.unplayed)}
      {playedClassName !== undefined && row(`${s.played} ${playedClassName}`)}
    </span>
  );
}

// The 44 mint circle of their note. Children replace the play glyph.
export function PlayButton({ children }: { children?: ReactNode }) {
  return <span className={s.play}>{children ?? <PlayGlyph />}</span>;
}

// The four steps of my note: sent, translating, voice, delivered. `done` are
// full; the next one fills with `fillClassName` when it is given.
export function SegBar({ done, fillClassName }: { done: number; fillClassName?: string }) {
  return (
    <span className={s.seg}>
      {[0, 1, 2, 3].map((i) => (
        <span key={i} className={s.segItem}>
          <span className={s.segTrack} />
          {i < done && <span className={s.segFill} />}
          {i === done && fillClassName && <span className={`${s.segFill} ${fillClassName}`} />}
        </span>
      ))}
    </span>
  );
}

// "Preparing your voice": a ring with a dot inside that breathes.
export function BreathDot() {
  return (
    <span className={s.breath}>
      <span className={s.breathDot} />
    </span>
  );
}

type RingsProps = {
  size: number;
  tone: "you" | "them";
  // Seconds per ring; the three rings are a third of it apart.
  period: number;
  // Shifts the whole set, in seconds (the welcome screen offsets mint by 0.5).
  offset?: number;
  // Still, the rings are a static halo, as Reduce Motion asks.
  playing?: boolean;
};

export function Rings({ size, tone, period, offset = 0, playing = true }: RingsProps) {
  const style = { width: size, height: size, "--period": `${period}s`, "--offset": `${offset}s` } as CSSProperties;
  return (
    <span className={`${s.rings} ${s[`${tone}Rings`]}`} style={style} data-playing={playing}>
      {[0, 1, 2].map((n) => (
        <span key={n} className={s.ring} style={{ "--n": n } as CSSProperties} />
      ))}
    </span>
  );
}
