"use client";

import { useEffect, useState } from "react";
import { APP_NAME } from "@/lib/site";
import { Lockup } from "./Lockup";
import s from "./SiteHeader.module.css";

const LINKS = [
  { href: "/#how", label: "How it works" },
  { href: "/#faq", label: "Questions" },
];

// The same header on every page, kept at the top, so the waitlist is one tap
// away wherever someone is. A hairline appears under it once the page has
// scrolled. `intro` plays the mark's intro, on the home page only.
export function SiteHeader({ intro = false }: { intro?: boolean }) {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const update = () => setScrolled(window.scrollY > 8);
    update();
    window.addEventListener("scroll", update, { passive: true });
    return () => window.removeEventListener("scroll", update);
  }, []);

  return (
    <header className={s.header} data-scrolled={scrolled}>
      <div className={s.inner}>
        <a href="/" className={s.home} aria-label={`${APP_NAME}, home`}>
          <Lockup intro={intro} />
        </a>
        <nav aria-label="On this site" className={s.nav}>
          <ul className={s.links}>
            {LINKS.map((link) => (
              <li key={link.href}>
                <a href={link.href}>{link.label}</a>
              </li>
            ))}
          </ul>
          <a className={s.cta} href="/#join">
            Try it first
          </a>
        </nav>
      </div>
    </header>
  );
}
