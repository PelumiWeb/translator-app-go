"use client";

import { LazyMotion, MotionConfig } from "framer-motion";
import type { ReactNode } from "react";
import { DURATION, EASE } from "@/lib/motion";

// Motion's features arrive in their own file once the page is up: everything
// they animate is below the hero, and until then `m` parts are drawn still.
// `strict` keeps the full `motion` component, which would load them all up
// front, out of the site. Every animation uses Pearl's easing, and under
// Reduce Motion, Motion keeps only fades (no movement, no layout changes).
//
// Motion is imported as `framer-motion`, its React package's first name. The
// newer `motion/react` imports all of it as one object, so the build cannot
// leave out what the site doesn't use: 57 KB more at first load, not 23 KB
// (ADR 0028, addendum of 2026-11-21).
const loadFeatures = () => import("@/lib/motion-features").then((module) => module.default);

export function MotionProvider({ children }: { children: ReactNode }) {
  return (
    <LazyMotion features={loadFeatures} strict>
      <MotionConfig reducedMotion="user" transition={{ duration: DURATION, ease: EASE }}>
        {children}
      </MotionConfig>
    </LazyMotion>
  );
}
