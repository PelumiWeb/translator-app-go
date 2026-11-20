import { Rings } from "./NoteParts";
import s from "./TwoVoices.module.css";

// The welcome screen's two voices: a mint and a warm-white ring set, 180
// each, 80 apart so they overlap; 3 s loops, mint half a second behind.
// Without `cores`, only the rings: a halo for something drawn over them.
export function TwoVoices({ cores = true }: { cores?: boolean }) {
  return (
    <div className={s.pair} aria-hidden="true">
      <span className={s.voice}>
        <Rings size={180} tone="them" period={3} offset={-0.5} />
        {cores && <span className={`${s.core} ${s.them}`} />}
      </span>
      <span className={`${s.voice} ${s.right}`}>
        <Rings size={180} tone="you" period={3} />
        {cores && <span className={`${s.core} ${s.you}`} />}
      </span>
    </div>
  );
}
