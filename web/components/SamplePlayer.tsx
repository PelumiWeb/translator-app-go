"use client";

import { useRef, useState } from "react";
import { PauseGlyph, PlayGlyph } from "./Icons";
import s from "./SamplePlayer.module.css";

type Phase = "idle" | "original" | "translated";

const LINES: Record<Phase, string> = {
  idle: "Spanish, heard in English, in her voice",
  original: "Español, as she said it",
  translated: "English, in her voice",
};

// The welcome screen's sample card: Rosa's own words, then the same words in
// English in her voice. Nothing plays until the visitor presses play.
export function SamplePlayer({ original, translated }: { original: string; translated: string }) {
  const [phase, setPhase] = useState<Phase>("idle");
  const first = useRef<HTMLAudioElement>(null);
  const second = useRef<HTMLAudioElement>(null);

  function start(audio: HTMLAudioElement | null, next: Phase) {
    if (!audio) return;
    audio.currentTime = 0;
    setPhase(next);
    audio.play().catch(() => setPhase("idle"));
  }

  function toggle() {
    if (phase === "idle") {
      start(first.current, "original");
    } else {
      first.current?.pause();
      second.current?.pause();
      setPhase("idle");
    }
  }

  return (
    <div className={s.card}>
      <button
        type="button"
        className={s.play}
        onClick={toggle}
        aria-label={phase === "idle" ? "Hear Rosa say hello" : "Stop"}
      >
        {phase === "idle" ? <PlayGlyph size={17} /> : <PauseGlyph size={17} />}
      </button>
      <div>
        <p className={s.title}>Hear Rosa say hello</p>
        <p className={s.line} aria-live="polite">
          {LINES[phase]}
        </p>
      </div>
      <audio ref={first} src={original} preload="none" onEnded={() => start(second.current, "translated")} />
      <audio ref={second} src={translated} preload="none" onEnded={() => setPhase("idle")} />
    </div>
  );
}
