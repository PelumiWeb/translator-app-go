import type { ReactNode } from "react";
import { CONTACT_EMAIL, PAGES_UPDATED } from "@/lib/site";
import { SiteFooter } from "./SiteFooter";
import { SiteHeader } from "./SiteHeader";
import s from "./TextPage.module.css";

// A page of plain reading: help, privacy and terms. Each gives the contact
// address, so the build stops if there is none rather than publish them
// without one.
export function TextPage({ title, children }: { title: string; children: ReactNode }) {
  if (!CONTACT_EMAIL) {
    throw new Error("CONTACT_EMAIL in web/lib/site.ts is empty: the help, privacy and terms pages need it");
  }
  return (
    <>
      <SiteHeader />
      <main className={s.main}>
        <h1 className={s.title}>{title}</h1>
        <p className={s.updated}>Updated {PAGES_UPDATED}</p>
        <div className={s.prose}>{children}</div>
      </main>
      <SiteFooter />
    </>
  );
}
