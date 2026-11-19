import { CONTACT_EMAIL } from "@/lib/site";

// The contact address as a link, wherever a page gives it.
export function Contact() {
  return <a href={`mailto:${CONTACT_EMAIL}`}>{CONTACT_EMAIL}</a>;
}

// A text with the contact address in it, the address made a link.
export function WithContact({ text }: { text: string }) {
  const at = CONTACT_EMAIL ? text.indexOf(CONTACT_EMAIL) : -1;
  if (at < 0) return <>{text}</>;
  return (
    <>
      {text.slice(0, at)}
      <Contact />
      {text.slice(at + CONTACT_EMAIL.length)}
    </>
  );
}
