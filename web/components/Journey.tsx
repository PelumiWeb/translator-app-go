"use client";

import { m, useScroll } from "framer-motion";
import { type CSSProperties, useEffect, useRef, useState } from "react";
import { STAGES } from "@/lib/site";
import { type Direction, StageFrame } from "./StageFrame";
import s from "./Journey.module.css";

// The journey, told while scrolling. The active step is the last one whose
// count ("3 of 7") has passed the middle of the window. On a wide screen one
// phone stays beside the steps and shows it, and a rail beside the steps
// fills as far as the middle of the window, lighting each step's point as it
// is reached; on a narrow one each step carries its own frame, which plays
// when its step is active. Scrolling back goes back; the page scrolls as
// usual. Worked out from where the steps are, not from which one crossed the
// middle, so a jump (a link to #join, then back up) lands on the right step.
export function Journey() {
  const [view, setView] = useState<{ active: number; direction: Direction }>({ active: 0, direction: 1 });
  const list = useRef<HTMLOListElement>(null);
  const counts = useRef<(HTMLParagraphElement | null)[]>([]);
  const rail = useRef<HTMLSpanElement>(null);
  // From the first step's point to the last one's, in the list.
  const [railBox, setRailBox] = useState<{ top: number; height: number } | null>(null);
  // 0 when the rail's top is at the middle of the window, 1 when its bottom is.
  const { scrollYProgress } = useScroll({ target: rail, offset: ["start center", "end center"] });

  useEffect(() => {
    let frame = 0;
    const middleOf = (el: Element) => {
      const box = el.getBoundingClientRect();
      return box.top + box.height / 2;
    };
    const update = () => {
      frame = 0;
      const middle = window.innerHeight / 2;
      let current = 0;
      counts.current.forEach((count, i) => {
        if (count && middleOf(count) < middle) current = i;
      });
      setView((previous) =>
        previous.active === current ? previous : { active: current, direction: current > previous.active ? 1 : -1 },
      );
    };
    const measure = () => {
      const first = counts.current[0];
      const last = counts.current[counts.current.length - 1];
      if (!list.current || !first || !last) return;
      const top = list.current.getBoundingClientRect().top;
      setRailBox({ top: middleOf(first) - top, height: middleOf(last) - middleOf(first) });
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(update);
    };
    // The steps change height when the window does, and when the font arrives.
    const resized = new ResizeObserver(() => {
      measure();
      schedule();
    });
    if (list.current) resized.observe(list.current);
    update();
    window.addEventListener("scroll", schedule, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      resized.disconnect();
      window.removeEventListener("scroll", schedule);
    };
  }, []);

  const { active, direction } = view;
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
        <div className={s.track}>
          <span
            ref={rail}
            className={s.rail}
            style={(railBox ? { top: railBox.top, height: railBox.height } : { visibility: "hidden" }) as CSSProperties}
            aria-hidden="true"
          >
            <m.span className={s.railFill} style={{ scaleY: scrollYProgress }} />
          </span>
          <ol ref={list} className={s.steps}>
            {STAGES.map((stage, i) => (
              <li key={stage.title} data-active={i === active} data-reached={i <= active} className={s.step}>
                <p
                  ref={(el) => {
                    counts.current[i] = el;
                  }}
                  className={s.count}
                >
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
        </div>
        <div className={s.side}>
          <div className={s.sticky}>
            <StageFrame stage={active} playing travel direction={direction} />
          </div>
        </div>
      </div>
    </section>
  );
}
