import { FAQ } from "@/lib/site";
import { WithContact } from "./Contact";
import { FaqItem } from "./FaqItem";
import s from "./Faq.module.css";

// The questions, each opening on its own (FaqItem).
export function Faq() {
  return (
    <ul className={s.list}>
      {FAQ.map(({ question, answer }) => (
        <li key={question} className={s.item}>
          <FaqItem question={question}>
            <p className={s.answer}>
              <WithContact text={answer} />
            </p>
          </FaqItem>
        </li>
      ))}
    </ul>
  );
}
