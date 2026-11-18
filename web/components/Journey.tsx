"use client";

import { useEffect, useRef, useState } from "react";
import { STAGES } from "@/lib/site";
import { StageFrame } from "./StageFrame";
import s from "./Journey.module.css";

// The journey, told while scrolling. The step in the middle of the window is
// the active one: on a wide screen one frame stays beside the steps and
// shows it; on a narrow one each step carries its own frame, which plays when
// its step is active. Scrolling back goes back; the page scrolls as usual.
export function Journey() {
  const [active, setActive] = useState(0);
  const steps = useRef<(HTMLLIElement | null)[]>([]);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) setActive(Number((entry.target as HTMLElement).dataset.index));
        }
      },
      // A thin band across the middle of the window.
      { rootMargin: "-45% 0px -45% 0px" },
    );
    for (const step of steps.current) if (step) observer.observe(step);
    return () => observer.disconnect();
  }, []);

  return (
    <section className={s.journey} aria-labelledby="journey-title">
      <div className={s.intro}>
        <h2 id="journey-title" className="title-2">
          How a voice note travels
        </h2>
        <p className="lede">
          Rosa, in Madrid, speaks Spanish. Her grandson, in Lagos, speaks English. This is what happens between them,
          in a few seconds.
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
