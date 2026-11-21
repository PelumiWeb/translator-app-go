"use client";

import { AnimatePresence, m, type Variants } from "framer-motion";
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
//
// With `travel`, the frame is the one beside the steps on a wide screen: the
// phone stays, and only its screen changes, rising or falling with the
// scroll. Rosa's note is one card from stage to stage: it moves and resizes
// to its place on the next screen, and changes side, from warm white (hers)
// to raised petrol (his), as her words become English.

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

type Tone = "mine" | "theirs";

// The note's card. Its colour is a surface behind the words, so that when the
// card travels between screens the surface can stretch and change colour
// while the words cross-fade, instead of being stretched with it. Only the
// travelling frame shares the surface; the frames inside each step (on a
// phone) must not, or they would all claim the same card.
function Card({ tone, travel, className = "", children }: { tone: Tone; travel: boolean; className?: string; children: ReactNode }) {
  return (
    <div className={`${s.card} ${s[tone]} ${className}`}>
      {travel ? (
        // Motion straightens the corners while the surface is resized, when
        // the radius is set here rather than in the stylesheet.
        <m.div layoutId="rosa-note" className={s.surface} style={{ borderRadius: 12 }} />
      ) : (
        <div className={s.surface} />
      )}
      <div className={s.content}>{children}</div>
    </div>
  );
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

function WriteDown({ travel }: { travel: boolean }) {
  return (
    <Card tone="mine" travel={travel}>
      <p className={s.text} lang="es">
        <Pieces items={SPANISH_WORDS} className={s.arriving} />
      </p>
      <p className={s.status}>Writing down what you said…</p>
    </Card>
  );
}

function Send({ travel }: { travel: boolean }) {
  return (
    <Card tone="mine" travel={travel}>
      <p className={s.text} lang="es">
        {ROSA.spanish}
      </p>
      <div className={s.barRow}>
        <SegBar done={0} fillClassName={s.filling} />
      </div>
      <p className={s.status}>Sending</p>
    </Card>
  );
}

function Translate({ travel }: { travel: boolean }) {
  return (
    <div className={s.column}>
      <Card tone="theirs" travel={travel}>
        <p className={s.langLabel}>Español</p>
        <p className={s.original} lang="es">
          {ROSA.spanish}
        </p>
        <hr className={s.hairline} />
        <p className={`${s.langLabel} ${s.them}`}>English</p>
        <p className={s.text}>
          <Pieces items={ROSA.englishPhrases} className={s.phrase} />
        </p>
      </Card>
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

function Arrive({ travel }: { travel: boolean }) {
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
      <Card tone="theirs" travel={travel} className={s.arrived}>
        <div className={s.playerRow}>
          <PlayButton />
          <Dots tone="them" />
          <span className={s.unread} />
        </div>
        <p className={s.text}>{ENGLISH}</p>
        <p className={s.meta}>New · {ROSA.duration} · in her voice</p>
      </Card>
    </div>
  );
}

function Hear({ travel }: { travel: boolean }) {
  return (
    <Card tone="theirs" travel={travel}>
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
    </Card>
  );
}

function screen(stage: number, playing: boolean, travel: boolean): ReactNode {
  switch (stage) {
    case 0:
      return <Speak playing={playing} />;
    case 1:
      return <WriteDown travel={travel} />;
    case 2:
      return <Send travel={travel} />;
    case 3:
      return <Translate travel={travel} />;
    case 4:
      return <Voice playing={playing} />;
    case 5:
      return <Arrive travel={travel} />;
    default:
      return <Hear travel={travel} />;
  }
}

// 1 when the journey goes forward (scrolling down), -1 going back. The next
// screen comes from the side the scroll is heading to, and the last one
// leaves the other way.
export type Direction = 1 | -1;

const SCENE: Variants = {
  enter: (direction: Direction) => ({ opacity: 0, y: 24 * direction }),
  shown: { opacity: 1, y: 0 },
  leave: (direction: Direction) => ({ opacity: 0, y: -24 * direction, transition: { duration: 0.3 } }),
};

const PLACE: Variants = {
  enter: { opacity: 0 },
  shown: { opacity: 1 },
  leave: { opacity: 0, transition: { duration: 0.2 } },
};

type Props = {
  stage: number;
  playing: boolean;
  travel?: boolean;
  direction?: Direction;
};

export function StageFrame({ stage, playing, travel = false, direction = 1 }: Props) {
  const place = stagePlace(stage);
  return (
    <div className={s.frame} data-playing={playing} aria-hidden="true">
      {travel ? (
        <div className={s.placeRow}>
          <AnimatePresence mode="popLayout" initial={false}>
            <m.p key={place} className={s.place} variants={PLACE} initial="enter" animate="shown" exit="leave">
              {place}
            </m.p>
          </AnimatePresence>
        </div>
      ) : (
        <p className={s.place}>{place}</p>
      )}
      <div className={s.screen}>
        {travel ? (
          // popLayout lifts the leaving screen out of the flow, so the next
          // one takes its place at once and the two cross.
          <AnimatePresence mode="popLayout" initial={false} custom={direction}>
            <m.div
              key={stage}
              className={s.scene}
              custom={direction}
              variants={SCENE}
              initial="enter"
              animate="shown"
              exit="leave"
            >
              {screen(stage, playing, true)}
            </m.div>
          </AnimatePresence>
        ) : (
          screen(stage, playing, false)
        )}
      </div>
      <div className={s.progress}>
        {STAGES.map((_, i) => (
          <span key={i} className={s.step} data-done={i <= stage} />
        ))}
      </div>
    </div>
  );
}
