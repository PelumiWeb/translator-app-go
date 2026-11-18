import { ROSA } from "@/lib/site";
import { BreathDot, Dots, PlayButton, SegBar } from "./NoteParts";
import s from "./HeroLoop.module.css";

// The core turn of the journey, on a loop: Rosa's note, warm white on her
// phone, changes language, waits for her voice, and arrives as theirs (mint)
// on her grandson's phone. One card whose colour changes side. CSS only; the
// still state, for Reduce Motion, is the note as he receives it.
export function HeroLoop() {
  return (
    <div className={s.loop} role="img" aria-label="Rosa's voice note, said in Spanish, arrives in English, in her own voice.">
      <div className={s.captions} aria-hidden="true">
        <p className={s.capA}>
          Rosa speaks <span className={s.lang}>Español</span>
        </p>
        <p className={s.capB}>
          Her grandson hears <span className={s.langThem}>English</span>
        </p>
      </div>
      <div className={s.card} aria-hidden="true">
        <div className={s.playerWrap}>
          <div className={s.playerInner}>
            <div className={s.player}>
              <PlayButton />
              <Dots tone="them" playedClassName={s.dotsPlayed} />
              <span className={s.duration}>{ROSA.duration}</span>
            </div>
          </div>
        </div>
        <div className={s.stacked}>
          <p className={s.es} lang="es">
            {ROSA.spanishShort}
          </p>
          <p className={s.en}>{ROSA.englishShort}</p>
        </div>
        <div className={`${s.stacked} ${s.statuses}`}>
          <div className={s.st1}>
            <SegBar done={1} fillClassName={s.segFill} />
            <span>Translating into English</span>
          </div>
          <div className={s.st2}>
            <BreathDot />
            <span>Preparing her voice in English</span>
          </div>
          <p className={s.st3}>In her voice</p>
        </div>
      </div>
    </div>
  );
}
