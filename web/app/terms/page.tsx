import type { Metadata } from "next";
import { Contact } from "@/components/Contact";
import { TextPage } from "@/components/TextPage";
import { APP_NAME, MINIMUM_AGE } from "@/lib/site";

export const metadata: Metadata = { title: `Terms · ${APP_NAME}` };

// The terms of use (ADR 0032). The App Store asks apps where people message
// each other to have terms with no tolerance for abuse, and a way to report
// it: in the app since checkpoint 16.2, and by email (ADR 0035).
export default function Terms() {
  return (
    <TextPage title="Terms of use">
      <p>
        These are the rules for using {APP_NAME}: the app and this website. By using {APP_NAME} you agree to them.
        &ldquo;We&rdquo; is {APP_NAME}; write to <Contact />. How we handle your data is on the{" "}
        <a href="/privacy">privacy page</a>.
      </p>

      <h2>Who can use it</h2>
      <p>
        You must be {MINIMUM_AGE} or over. Your account is yours alone: don&apos;t share it, or use someone else&apos;s.
      </p>

      <h2>It&apos;s a test</h2>
      <p>
        {APP_NAME} is being tested, and is free while we test. Things will change, break or stop working, and messages
        can be lost. Each person can send 10 minutes of voice notes and 100 text messages in any 24 hours, and
        translation runs on a monthly allowance shared by every tester. We can change these limits, or end the test, at
        any time.
      </p>

      <h2>Only your own voice</h2>
      <p>
        Record only your own voice, reading the passage the app gives you. Never record someone else or play a recording
        of them, and never use {APP_NAME} to make anyone believe someone said something they didn&apos;t. Each
        person&apos;s voice is made only from the recording they gave, with their agreement, and only to speak their own
        words.
      </p>

      <h2>How to treat people</h2>
      <p>We have no tolerance for abuse or objectionable content. Don&apos;t use {APP_NAME} to:</p>
      <ul>
        <li>harass, threaten, bully or frighten anyone</li>
        <li>send hateful, violent or sexual content</li>
        <li>involve a child in anything sexual, ever</li>
        <li>pretend to be someone else</li>
        <li>send spam, scams, or anything illegal</li>
        <li>get around the limits, or break or overload {APP_NAME}</li>
      </ul>

      <h2>Reporting someone</h2>
      <p>
        If someone breaks these rules, report them in the app: press and hold their message, or use the menu at the top
        of the room. You can also write to <Contact /> with their ID and what happened. We look into every report within
        24 hours, and we delete what breaks these rules and close the accounts that send it.
      </p>

      <h2>What you say</h2>
      <p>
        What you say and write stays yours. You let us keep it, write it down, translate it and speak it in your voice,
        only to carry it to the people in your rooms, as the privacy page describes. You&apos;re responsible for what you
        send.
      </p>

      <h2>Translations can be wrong</h2>
      <p>
        {APP_NAME}&apos;s translations and voices are made by machines. They can be wrong, miss words, or change what you
        meant. Don&apos;t rely on {APP_NAME} where a mistake could hurt someone: for medical, legal or safety matters, or
        in an emergency.
      </p>

      <h2>Stopping</h2>
      <p>
        You can stop at any time: delete your account in Profile. We can suspend or close an account that breaks these
        terms, and we can end the test.
      </p>

      <h2>No promises beyond these</h2>
      <p>
        {APP_NAME} is offered as it is, while we test it, with no promise that it will work, be available, or suit what
        you need. As far as the law allows, we are not responsible for any loss that comes from using it, or from not
        being able to. Nothing here takes away rights the law gives you.
      </p>

      <h2>Apple and Google</h2>
      <p>
        On iPhone, Apple&apos;s standard licence agreement for apps, and Apple&apos;s terms for TestFlight, also apply.
        Apple and Google are not responsible for {APP_NAME}, or for supporting it.
      </p>

      <h2>Changes</h2>
      <p>
        When these terms change, the date at the top changes too. If a change matters, we&apos;ll say so here and in a
        new version&apos;s notes before it takes effect. Using {APP_NAME} after that means you agree to the new terms.
      </p>
    </TextPage>
  );
}
