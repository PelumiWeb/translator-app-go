import { type CSSProperties, Fragment, type ReactNode } from "react";
import { ROSA, STAGES, stagePlace } from "@/lib/site";
import { Mark } from "./Lockup";
import { PauseGlyph, PlayGlyph, SpeakerIcon } from "./Icons";
import { Dots, PlayButton, Rings, SegBar } from "./NoteParts";
import s from "./StageFrame.module.css";

// One stage of the journey, drawn as the screen it happens on, with the
// app's own states (docs/design/pearl/v1/12-voice-note-states.png). Each
// stage plays once when `playing` turns on; its still state is where the
// stage ends, which is also what Reduce Motion shows.

const SPANISH_WORDS = ROSA.spanish.split(" ");
const ENGLISH = ROSA.englishPhrases.join(" ");
const ENGLISH_WORDS = ENGLISH.split(" ");

const order = (i: number) => ({ "--i": i }) as CSSProperties;

// Words or phrases as separate spans, so each can arrive on its own.
function Pieces({ items, className }: { items: string[]; className: string }) {
  return items.map((item, i) => (
    <Fragment key={i}>
      <span className={className} style={order(i)}>
        {item}
      </span>{" "}
    </Fragment>
  ));
}

function Speak({ playing }: { playing: boolean }) {
  return (
    <div className={s.stack}>
      <p className={s.label}>Listening</p>
      <p className={s.timer}>0:07</p>
      <div className={s.recorder}>
        <Rings size={200} tone="you" period={2.4} playing={playing} />
        <span className={s.recordButton}>
          <span className={s.stopGlyph} />
        </span>
      </div>
    </div>
  );
}

function WriteDown() {
  return (
    <div className={`${s.card} ${s.mine}`}>
      <p className={s.text} lang="es">
        <Pieces items={SPANISH_WORDS} className={s.arriving} />
      </p>
      <p className={s.status}>Writing down what you said…</p>
    </div>
  );
}

function Send() {
  return (
    <div className={`${s.card} ${s.mine}`}>
      <p className={s.text} lang="es">
        {ROSA.spanish}
      </p>
      <div className={s.barRow}>
        <SegBar done={0} fillClassName={s.filling} />
      </div>
      <p className={s.status}>Sending</p>
    </div>
  );
}

function Translate() {
  return (
    <div className={s.column}>
      <div className={`${s.card} ${s.theirs}`}>
        <p className={s.langLabel}>Español</p>
        <p className={s.original} lang="es">
          {ROSA.spanish}
        </p>
        <hr className={s.hairline} />
        <p className={`${s.langLabel} ${s.them}`}>English</p>
        <p className={s.text}>
          <Pieces items={ROSA.englishPhrases} className={s.phrase} />
        </p>
      </div>
      <div className={s.under}>
        <SegBar done={1} fillClassName={s.fillingSlow} />
        <p className={s.status}>Translating into English</p>
      </div>
    </div>
  );
}

function Voice({ playing }: { playing: boolean }) {
  return (
    <div className={s.stack}>
      <div className={s.voice}>
        <Rings size={220} tone="you" period={3.2} playing={playing} />
        <span className={s.voiceButton}>
          <SpeakerIcon size={30} />
        </span>
      </div>
      <p className={s.headline}>Preparing her voice in English</p>
      <p className={s.sub}>Usually a few seconds</p>
    </div>
  );
}

function Arrive() {
  return (
    <div className={s.column}>
      <div className={s.notification}>
        <span className={s.appIcon}>
          <Mark width={26} />
        </span>
        <span className={s.notifText}>
          <span className={s.notifTitle}>Rosa</span>
          <span className={s.notifBody}>Sent a voice note · {ROSA.duration}</span>
        </span>
        <span className={s.notifTime}>now</span>
      </div>
      <div className={`${s.card} ${s.theirs} ${s.arrived}`}>
        <div className={s.playerRow}>
          <PlayButton />
          <Dots tone="them" />
          <span className={s.unread} />
        </div>
        <p className={s.text}>{ENGLISH}</p>
        <p className={s.meta}>New · {ROSA.duration} · in her voice</p>
      </div>
    </div>
  );
}

function Hear() {
  return (
    <div className={`${s.card} ${s.theirs}`}>
      <div className={s.playerRow}>
        <PlayButton>
          <span className={s.whilePlaying}>
            <PauseGlyph />
          </span>
          <span className={s.afterPlaying}>
            <PlayGlyph />
          </span>
        </PlayButton>
        <Dots tone="them" playedClassName={s.dotsPlaying} />
        <span className={s.duration}>{ROSA.duration}</span>
      </div>
      <p className={s.text}>
        <Pieces items={ENGLISH_WORDS} className={s.lit} />
      </p>
      <div className={s.originalWrap}>
        <div className={s.originalInner}>
          <hr className={s.hairline} />
          <p className={`${s.langLabel} ${s.them}`}>Original · Español</p>
          <p className={s.original} lang="es">
            {ROSA.spanish}
          </p>
        </div>
      </div>
      <div className={s.metaStack}>
        <p className={`${s.meta} ${s.them} ${s.whilePlaying}`}>Playing in her voice</p>
        <p className={`${s.meta} ${s.afterPlaying}`}>
          Played · <span className={s.link}>Hide original</span>
        </p>
      </div>
    </div>
  );
}

function screen(stage: number, playing: boolean): ReactNode {
  switch (stage) {
    case 0:
      return <Speak playing={playing} />;
    case 1:
      return <WriteDown />;
    case 2:
      return <Send />;
    case 3:
      return <Translate />;
    case 4:
      return <Voice playing={playing} />;
    case 5:
      return <Arrive />;
    default:
      return <Hear />;
  }
}

export function StageFrame({ stage, playing }: { stage: number; playing: boolean }) {
  return (
    <div className={s.frame} data-playing={playing} aria-hidden="true">
      <p className={s.place}>{stagePlace(stage)}</p>
      <div className={s.screen}>{screen(stage, playing)}</div>
      <div className={s.progress}>
        {STAGES.map((_, i) => (
          <span key={i} className={s.step} data-done={i <= stage} />
        ))}
      </div>
    </div>
  );
}
