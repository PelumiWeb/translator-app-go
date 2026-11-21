"use client";

import { animate } from "framer-motion/dom/mini";
import { type MouseEvent, type ReactNode, useRef } from "react";
import { DURATION, EASE } from "@/lib/motion";
import s from "./Faq.module.css";

// One question. It is the browser's own disclosure, which keyboards and
// screen readers already know, and which opens with no script at all. With
// the script, the answer slides open and shut instead of appearing at once:
// the click is taken over, the height is animated, and the disclosure is
// opened before the slide or closed after it. A second click mid-slide turns
// it round from where it is. `data-closing` shows the plus while the answer
// is still closing. Reduce Motion: left to the browser.
export function FaqItem({ question, children }: { question: string; children: ReactNode }) {
  const details = useRef<HTMLDetailsElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const slide = useRef<ReturnType<typeof animate> | null>(null);

  function toggle(event: MouseEvent) {
    const el = details.current;
    const body = panel.current;
    if (!el || !body || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    event.preventDefault();

    const from = el.open ? body.getBoundingClientRect().height : 0;
    const closing = el.open && !("closing" in el.dataset);
    slide.current?.stop();
    if (closing) {
      el.dataset.closing = "";
    } else {
      delete el.dataset.closing;
      el.open = true; // before measuring: a closed disclosure's answer has no height
    }

    // Clipped only while it slides, so a focused link's outline shows after.
    body.style.overflow = "hidden";
    const to = closing ? 0 : body.scrollHeight;
    const run = animate(body, { height: [`${from}px`, `${to}px`] }, { duration: DURATION, ease: EASE });
    slide.current = run;
    run.then(() => {
      if (slide.current !== run) return; // turned round mid-slide
      if (closing) {
        el.open = false;
        delete el.dataset.closing;
      }
      body.style.height = "";
      body.style.overflow = "";
    });
  }

  return (
    <details ref={details} className={s.details}>
      <summary className={s.question} onClick={toggle}>
        {question}
        <span className={s.sign} aria-hidden="true" />
      </summary>
      <div ref={panel} className={s.panel}>
        {children}
      </div>
    </details>
  );
}
