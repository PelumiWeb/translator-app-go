import { NAME_STORY } from "@/lib/site";
import { Lockup } from "./Lockup";
import s from "./SiteFooter.module.css";

const LINKS = [
  { href: "/#faq", label: "Questions" },
  { href: "/support", label: "Help" },
  { href: "/privacy", label: "Privacy" },
  { href: "/terms", label: "Terms" },
];

// The same footer on every page: where help, privacy and terms are, which
// the app stores ask to be easy to find.
export function SiteFooter() {
  return (
    <footer className={s.footer}>
      <Lockup size="small" />
      <nav aria-label="About the app">
        <ul className={s.links}>
          {LINKS.map((link) => (
            <li key={link.href}>
              <a href={link.href}>{link.label}</a>
            </li>
          ))}
        </ul>
      </nav>
      <p className={s.story}>{NAME_STORY}</p>
    </footer>
  );
}
