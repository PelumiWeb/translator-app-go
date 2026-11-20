import { existsSync } from "node:fs";
import { join } from "node:path";
import type { CSSProperties } from "react";
import { HeroLoop } from "@/components/HeroLoop";
import { Faq } from "@/components/Faq";
import { Journey } from "@/components/Journey";
import { Reveals } from "@/components/Reveals";
import { SamplePlayer } from "@/components/SamplePlayer";
import { SiteFooter } from "@/components/SiteFooter";
import { SiteHeader } from "@/components/SiteHeader";
import { TwoVoices } from "@/components/TwoVoices";
import { Waitlist } from "@/components/Waitlist";
import { Ways } from "@/components/Ways";
import { Who } from "@/components/Who";
import { APP_NAME, HERO_NOTE, PROMISE, PROMISE_LINE, SAMPLE, WAITLIST } from "@/lib/site";
import s from "./page.module.css";

// The page is rendered once, at build time, so this looks at the files then.
function hasSample(): boolean {
  return [SAMPLE.original, SAMPLE.translated].every((file) => existsSync(join(process.cwd(), "public", file)));
}

const VOICE_BLOCKS = [
  {
    title: "What we keep",
    text: "The recording you make when you join, on our server, with the date you agreed.",
  },
  {
    title: "What it's for",
    text: "Only to speak your own words, when someone plays a message you sent. It is never given to other people.",
  },
  {
    title: "Delete it any time",
    text: "In Profile, under Your voice. Your messages then use the phone's voice. Delete your account, and your voice goes with it.",
  },
];

export default function Home() {
  return (
    <>
      <SiteHeader intro />

      <main>
        <section className={s.hero}>
          <div className={s.heroText}>
            <h1 className={s.promise}>{PROMISE}</h1>
            <p className={s.promiseLine}>{PROMISE_LINE}</p>
            {hasSample() && <SamplePlayer original={SAMPLE.original} translated={SAMPLE.translated} />}
            <div className={s.actions}>
              <a className={s.cta} href="#join">
                Try it first
              </a>
              <a className={s.secondary} href="#how">
                See how it works
              </a>
            </div>
            <p className={s.heroNote}>{HERO_NOTE}</p>
          </div>
          <div className={s.heroVisual}>
            {/* The Welcome screen's two voices, large and behind the loop. */}
            <div className={s.halo}>
              <TwoVoices cores={false} />
            </div>
            <HeroLoop />
          </div>
        </section>

        <Who />

        <Journey />

        <Ways />

        <section className={s.voice} aria-labelledby="voice-title">
          <h2 id="voice-title" className="title-2" data-reveal>
            Your voice stays yours.
          </h2>
          <p className="lede" data-reveal>
            To speak your words in your voice, {APP_NAME} keeps a recording of it, only with your agreement. This is
            exactly what that means.
          </p>
          <ul className={s.blocks}>
            {VOICE_BLOCKS.map((block, i) => (
              <li key={block.title} className={s.block} data-reveal style={{ "--i": i } as CSSProperties}>
                <h3 className={s.blockTitle}>{block.title}</h3>
                <p className={s.blockText}>{block.text}</p>
              </li>
            ))}
          </ul>
          <p className={s.noVoice} data-reveal>
            Rather not record your voice? You can still talk: people hear your messages in their phone&apos;s voice.
          </p>
        </section>

        <section id="faq" className={s.faq} aria-labelledby="faq-title">
          <h2 id="faq-title" className="title-2" data-reveal>
            Questions
          </h2>
          <Faq />
        </section>

        <section id="join" className={s.closing} aria-labelledby="closing-title">
          <TwoVoices />
          <h2 id="closing-title" className={`title-2 ${s.closingTitle}`}>
            {WAITLIST.title}
          </h2>
          <p className={`lede ${s.closingLine}`}>{WAITLIST.line}</p>
          <ol className={s.steps}>
            {WAITLIST.steps.map((step, i) => (
              <li key={step} className={s.stepItem} data-reveal style={{ "--i": i } as CSSProperties}>
                <span className={s.stepNumber}>{i + 1}</span>
                {step}
              </li>
            ))}
          </ol>
          <Waitlist />
        </section>
      </main>

      <SiteFooter />
      <Reveals />
    </>
  );
}
