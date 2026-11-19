import type { Metadata } from "next";
import { Contact } from "@/components/Contact";
import { TextPage } from "@/components/TextPage";
import { APP_NAME } from "@/lib/site";

export const metadata: Metadata = { title: `Help · ${APP_NAME}` };

// The support page the App Store asks for (ADR 0032): how to reach us, and
// how to do what people most often need help with. Deleting an account is
// here too, for Google Play, which asks for it on the web.
export default function Support() {
  return (
    <TextPage title="Help">
      <p>
        Write to <Contact /> with a question, a problem, or to report someone. The questions people ask most are answered
        on the <a href="/#faq">home page</a>.
      </p>

      <h2>Joining</h2>
      <ol>
        <li>
          Leave your email on the <a href="/#join">waitlist</a>, choosing iPhone.
        </li>
        <li>When your turn comes, an email arrives from TestFlight. Open it on your iPhone.</li>
        <li>
          Install TestFlight from the App Store if you don&apos;t have it, then accept the invitation and install{" "}
          {APP_NAME}. It needs iOS 18 or later: the iPhone XS and XR and everything newer.
        </li>
        <li>Open {APP_NAME}, sign in with Apple, and choose your name, ID and language.</li>
        <li>Record your voice (about 20 seconds), or use the phone&apos;s voice instead.</li>
        <li>Add the person you want to talk to by their ID, or scan their code, and send your first voice note.</li>
      </ol>

      <h2>Telling us what broke</h2>
      <p>
        In the TestFlight app, open {APP_NAME} and choose Send Beta Feedback. Or take a screenshot in {APP_NAME} and share
        it as beta feedback. Either way it reaches us with the screenshot. Tell us which languages you used.
      </p>

      <h2>Reporting someone</h2>
      <p>
        Write to <Contact /> with their ID and what happened. We look into every report within 24 hours. The{" "}
        <a href="/terms">terms</a> say what isn&apos;t allowed.
      </p>

      <h2>Deleting your voice</h2>
      <p>
        In Profile, under Your voice, choose Delete my voice. It is deleted from our server and your phone, and your
        messages use the phone&apos;s voice instead. You can record again at any time.
      </p>

      <h2>Deleting your account</h2>
      <p>
        In Profile, choose Delete account. Your profile, your voice and everything you said in your rooms are deleted,
        and this can&apos;t be undone. If you can&apos;t open the app, write to <Contact /> with your ID, and we&apos;ll
        delete it for you.
      </p>

      <h2>Leaving the test</h2>
      <p>
        Choose Stop Testing on {APP_NAME}&apos;s page in TestFlight. To delete what we keep about you as well, delete your
        account first.
      </p>

      <h2>Coming off the waitlist</h2>
      <p>
        Write to <Contact /> from the address you joined with.
      </p>
    </TextPage>
  );
}
