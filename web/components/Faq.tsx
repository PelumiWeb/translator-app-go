import { FAQ } from "@/lib/site";
import { WithContact } from "./Contact";
import s from "./Faq.module.css";

// Each question opens on its own, with no script: the browser's own
// disclosure, which keyboards and screen readers already know.
export function Faq() {
  return (
    <ul className={s.list}>
      {FAQ.map(({ question, answer }) => (
        <li key={question} className={s.item}>
          <details>
            <summary className={s.question}>{question}</summary>
            <p className={s.answer}>
              <WithContact text={answer} />
            </p>
          </details>
        </li>
      ))}
    </ul>
  );
}
