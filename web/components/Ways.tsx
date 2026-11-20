import type { CSSProperties, ReactNode } from "react";
import { ROSA, WAYS, type Way } from "@/lib/site";
import { SpeakerIcon, SwapIcon } from "./Icons";
import { Dots, PlayButton } from "./NoteParts";
import s from "./Ways.module.css";

// The three ways to talk, each with a glimpse of its screen, drawn from the
// app's own parts: their voice note, their text message, the interpreter's
// two languages. Rosa's words, as everywhere on the page.
export function Ways() {
  return (
    <section className={s.ways} aria-labelledby="ways-title">
      <h2 id="ways-title" className="title-2" data-reveal>
        {WAYS.title}
      </h2>
      <p className="lede" data-reveal>
        {WAYS.line}
      </p>
      <ul className={s.cards}>
        <Card way={WAYS.voice} index={0}>
          <div className={s.note}>
            <div className={s.player}>
              <PlayButton />
              <Dots tone="them" playedClassName={s.half} />
              <span className={s.duration}>{ROSA.duration}</span>
            </div>
            <p className={s.words}>{ROSA.englishShort}</p>
            <p className={s.meta}>
              In her voice · <span className={s.link}>Show original</span>
            </p>
          </div>
        </Card>
        <Card way={WAYS.text} index={1}>
          <div className={s.note}>
            <p className={s.words}>{ROSA.englishPhrases[0]}</p>
            <p className={s.listen}>
              <span className={s.listenButton}>
                <SpeakerIcon size={16} />
              </span>
              <span className={s.link}>Show original</span>
            </p>
          </div>
        </Card>
        <Card way={WAYS.interpreter} index={2}>
          <div className={s.interpreter}>
            <p className={s.chips}>
              <span className={s.chipYou}>Español</span>
              <SwapIcon />
              <span className={s.chipThem}>English</span>
            </p>
            <p className={s.said} lang="es">
              {ROSA.spanishShort}
            </p>
            <p className={s.heard}>{ROSA.englishShort}</p>
          </div>
        </Card>
      </ul>
    </section>
  );
}

function Card({ way, index, children }: { way: Way; index: number; children: ReactNode }) {
  return (
    <li className={s.card} data-reveal style={{ "--i": index } as CSSProperties}>
      <div className={s.glimpse} aria-hidden="true">
        {children}
      </div>
      <h3 className={s.title}>{way.title}</h3>
      <p className={s.text}>{way.text}</p>
    </li>
  );
}
