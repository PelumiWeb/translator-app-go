"use client";

import { useEffect, useRef, useState } from "react";
import { STAGES } from "@/lib/site";
import { StageFrame } from "./StageFrame";
import s from "./Journey.module.css";

// The journey, told while scrolling. The active step is the last one whose
// top has passed the middle of the window: on a wide screen one frame stays
// beside the steps and shows it; on a narrow one each step carries its own
// frame, which plays when its step is active. Scrolling back goes back; the
// page scrolls as usual. Worked out from where the steps are, not from which
// one crossed the middle, so a jump (a link to #join, then back up) lands on
// the right step too.
export function Journey() {
  const [active, setActive] = useState(0);
  const steps = useRef<(HTMLLIElement | null)[]>([]);

  useEffect(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      const middle = window.innerHeight / 2;
      let current = 0;
      steps.current.forEach((step, i) => {
        if (step && step.getBoundingClientRect().top < middle) current = i;
      });
      setActive(current);
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    update();
    window.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("resize", schedule);
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
    };
  }, []);

  return (
    <section id="how" className={s.journey} aria-labelledby="journey-title">
      <div className={s.intro} data-reveal>
        <h2 id="journey-title" className="title-2">
          How a voice note crosses the language barrier
        </h2>
        <p className="lede">
          Rosa, in Madrid, speaks Spanish. Her grandson, in Lagos, speaks English. They don&apos;t share a language.
          This is what happens between them, in a few seconds.
        </p>
      </div>
      <div className={s.layout}>
        <ol className={s.steps}>
          {STAGES.map((stage, i) => (
            <li
              key={stage.title}
              ref={(el) => {
                steps.current[i] = el;
              }}
              data-index={i}
              data-active={i === active}
              className={s.step}
            >
              <p className={s.count}>
                {i + 1} of {STAGES.length}
              </p>
              <h3 className={s.title}>{stage.title}</h3>
              <p className={s.body}>{stage.body}</p>
              <p className={s.time}>{stage.time}</p>
              <div className={s.inlineFrame}>
                <StageFrame stage={i} playing={i === active} />
              </div>
            </li>
          ))}
        </ol>
        <div className={s.side}>
          <div className={s.sticky}>
            {/* A new key on each stage starts its animations from the beginning. */}
            <div key={active} className={s.swap}>
              <StageFrame stage={active} playing />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
