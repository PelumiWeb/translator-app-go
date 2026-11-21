"use client";

import { AnimatePresence, m } from "framer-motion";
import { useState, type FormEvent } from "react";
import { CONTACT_EMAIL, WAITLIST } from "@/lib/site";
import s from "./Waitlist.module.css";

type Phone = "iphone" | "android";
type State = { kind: "idle" } | { kind: "sending" } | { kind: "joined"; phone: Phone } | { kind: "failed"; message: string };

const PHONES: { value: Phone; label: string }[] = [
  { value: "iphone", label: "iPhone" },
  { value: "android", label: "Android" },
];

export function Waitlist() {
  const [phone, setPhone] = useState<Phone>("iphone");
  const [state, setState] = useState<State>({ kind: "idle" });

  async function join(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setState({ kind: "sending" });
    try {
      const response = await fetch(WAITLIST.path, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: form.get("email"),
          name: form.get("name"),
          phone,
          company: form.get("leave_empty"), // the server's name for the bot trap (ADR 0030)
        }),
      });
      if (response.ok) {
        setState({ kind: "joined", phone });
        return;
      }
      const body = await response.json().catch(() => null);
      const code = body?.error?.code;
      setState({
        kind: "failed",
        message: code === "invalid_email" ? "That doesn't look like an email address." : "Something went wrong on our side. Try again in a moment.",
      });
    } catch {
      setState({ kind: "failed", message: "We couldn't reach the list. Check your connection and try again." });
    }
  }

  const sending = state.kind === "sending";
  const form = (
    <m.form key="form" className={s.form} onSubmit={join} exit={{ opacity: 0, y: -8, transition: { duration: 0.25 } }}>
      <div>
        {/* The chosen phone's warm-white capsule slides across to the other
            choice, under its name. */}
        <fieldset className={s.phones}>
          <legend className="visually-hidden">Your phone</legend>
          {PHONES.map((option) => (
            <label key={option.value} className={s.phone} data-selected={phone === option.value}>
              <input
                type="radio"
                name="phone"
                value={option.value}
                checked={phone === option.value}
                onChange={() => setPhone(option.value)}
                className="visually-hidden"
              />
              {phone === option.value && (
                <m.span layoutId="waitlist-phone" className={s.chosen} style={{ borderRadius: 999 }} transition={{ duration: 0.35 }} />
              )}
              <span className={s.phoneName}>{option.label}</span>
            </label>
          ))}
        </fieldset>
        <AnimatePresence initial={false}>
          {phone === "android" && (
            <m.div
              key="android"
              className={s.noteWrap}
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: "auto", opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
            >
              <p className={s.note}>{WAITLIST.android}</p>
            </m.div>
          )}
        </AnimatePresence>
      </div>

      <label className={s.field}>
        <span className={s.label}>Your email</span>
        <input className={s.input} type="email" name="email" required autoComplete="email" inputMode="email" maxLength={254} />
      </label>
      <label className={s.field}>
        <span className={s.label}>Your first name (optional)</span>
        <input className={s.input} type="text" name="name" autoComplete="given-name" maxLength={50} />
      </label>
      {/* Hidden from people and from screen readers: only a bot fills it in.
          Its name and label match nothing a browser autofills (a "company"
          field would be filled with the visitor's employer, and their sign-up
          dropped), and anyone who reads the page's text is told to leave it. */}
      <div className={s.trap} aria-hidden="true">
        <label>
          Leave this empty
          <input type="text" name="leave_empty" tabIndex={-1} autoComplete="off" />
        </label>
      </div>

      <button className={s.button} type="submit" disabled={sending}>
        {sending ? "Joining…" : "Join the list"}
      </button>
      {state.kind === "failed" && (
        <p className={s.failed} role="alert">
          {state.message}
        </p>
      )}
      <p className={s.privacy}>
        {WAITLIST.privacy}{" "}
        {CONTACT_EMAIL ? (
          <>
            To be taken off the list, write to <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>.
          </>
        ) : (
          "To be taken off the list, reply to any email from us."
        )}
      </p>
    </m.form>
  );

  // Once joined, the form gives way to the thanks: it fades and rises out,
  // then the thanks rises in, the page's own entrance.
  return (
    <AnimatePresence mode="wait" initial={false}>
      {state.kind === "joined" ? (
        <m.p key="joined" className={s.joined} role="status" initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }}>
          {state.phone === "iphone" ? WAITLIST.joinedIPhone : WAITLIST.joinedAndroid}
        </m.p>
      ) : (
        form
      )}
    </AnimatePresence>
  );
}
