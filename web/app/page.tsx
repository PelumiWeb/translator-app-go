import { existsSync } from "node:fs";
import { join } from "node:path";
import { HeroLoop } from "@/components/HeroLoop";
import { Journey } from "@/components/Journey";
import { Lockup } from "@/components/Lockup";
import { SamplePlayer } from "@/components/SamplePlayer";
import { TwoVoices } from "@/components/TwoVoices";
import { APP_NAME, NAME_STORY, PROMISE, PROMISE_LINE, SAMPLE } from "@/lib/site";
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
      <header className={s.header}>
        <Lockup />
      </header>

      <main>
        <section className={s.hero}>
          <div className={s.heroText}>
            <h1 className={s.promise}>{PROMISE}</h1>
            <p className={s.promiseLine}>{PROMISE_LINE}</p>
            {hasSample() && <SamplePlayer original={SAMPLE.original} translated={SAMPLE.translated} />}
            <p className={s.soon}>Coming to iPhone and Android</p>
          </div>
          <div className={s.heroVisual}>
            <HeroLoop />
          </div>
        </section>

        <Journey />

        <section className={s.voice} aria-labelledby="voice-title">
          <h2 id="voice-title" className="title-2">
            Your voice stays yours.
          </h2>
          <p className="lede">
            To speak your words in your voice, {APP_NAME} keeps a recording of it, only with your agreement. This is
            exactly what that means.
          </p>
          <ul className={s.blocks}>
            {VOICE_BLOCKS.map((block) => (
              <li key={block.title} className={s.block}>
                <h3 className={s.blockTitle}>{block.title}</h3>
                <p className={s.blockText}>{block.text}</p>
              </li>
            ))}
          </ul>
          <p className={s.noVoice}>
            Rather not record your voice? You can still talk: people hear your messages in their phone&apos;s voice.
          </p>
        </section>

        <section className={s.closing} aria-labelledby="closing-title">
          <TwoVoices />
          <h2 id="closing-title" className={`title-2 ${s.closingTitle}`}>
            {PROMISE}
          </h2>
          <p className={s.soon}>Coming to iPhone and Android</p>
        </section>
      </main>

      <footer className={s.footer}>
        <Lockup size="small" />
        <p className={s.story}>{NAME_STORY}</p>
      </footer>
    </>
  );
}
