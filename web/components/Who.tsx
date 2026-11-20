import type { CSSProperties } from "react";
import { WHO } from "@/lib/site";
import s from "./Who.module.css";

// Who it's for, by relationship rather than by use: the people Shinar is
// built around are ones you already know (marketing brief, section 3).
export function Who() {
  return (
    <section className={s.who} aria-labelledby="who-title">
      <h2 id="who-title" className="title-2" data-reveal>
        {WHO.title}
      </h2>
      <ul className={s.examples}>
        {WHO.examples.map((example, i) => (
          <li key={example.lead} className={s.example} data-reveal style={{ "--i": i } as CSSProperties}>
            <span className={s.lead}>{example.lead}</span> {example.rest}
          </li>
        ))}
      </ul>
      <p className={s.line} data-reveal>
        {WHO.line}
      </p>
    </section>
  );
}
