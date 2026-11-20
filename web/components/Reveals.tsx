"use client";

import { useEffect } from "react";

// Parts marked `data-reveal` rise 8 and fade in as they come into view, the
// wordmark's own entrance (Pearl, "Splash"), once each. Nothing is hidden
// until this has run, so the page reads the same without JavaScript, and what
// is already on screen is shown before the rest is hidden, so it never
// flickers. Reduce Motion: nothing moves or hides. The styles are in
// globals.css; `--i` on a part delays it by that many steps.
export function Reveals() {
  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const parts = Array.from(document.querySelectorAll<HTMLElement>("[data-reveal]"));
    const show = (el: Element) => el.setAttribute("data-shown", "");
    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            show(entry.target);
            observer.unobserve(entry.target);
          }
        }
      },
      { rootMargin: "0px 0px -8% 0px" },
    );
    for (const el of parts) {
      // In view, or already scrolled past (a link to #join, say): shown now.
      if (el.getBoundingClientRect().top < window.innerHeight) show(el);
      else observer.observe(el);
    }
    document.documentElement.classList.add("reveals");
    return () => observer.disconnect();
  }, []);

  return null;
}
