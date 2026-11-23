import type { Metadata } from "next";
import { Contact } from "@/components/Contact";
import { TextPage } from "@/components/TextPage";
import { APP_NAME, MINIMUM_AGE } from "@/lib/site";

export const metadata: Metadata = { title: `Privacy · ${APP_NAME}` };

// The privacy policy the App Store and Google Play ask for (ADR 0032). Every
// line is what the app and the server do today; it names the services the
// app's own privacy page names. Change it when they change.
export default function Privacy() {
  return (
    <TextPage title="Privacy">
      <p>
        This is what {APP_NAME} keeps about you, why, who helps us do it, and how to delete it. It covers the app, on
        iPhone and Android, and this website. {APP_NAME} is being tested, and is not on the App Store or Google Play
        yet. &ldquo;We&rdquo; is {APP_NAME}. Write to <Contact /> with any question.
      </p>

      <h2>In short</h2>
      <ul>
        <li>We keep what the app needs to carry your words to the people you talk to, in their language and your voice.</li>
        <li>Your voice is kept only with your agreement, used only to speak your own words, and you can delete it any time.</li>
        <li>Messages are not end-to-end encrypted: our server reads the words in order to translate them.</li>
        <li>We never sell your data. There are no ads, and nothing tracks you across other apps or websites.</li>
      </ul>

      <h2>Your account</h2>
      <p>
        Your name, your ID and the language you speak, so people can find you and hear you in their language. You sign
        in with Apple on iPhone, or Google on Android. We keep your email address only if Apple shares it or Google
        confirms it, to recognise your account.
      </p>

      <h2>Your voice</h2>
      <p>
        When you join, you can record yourself reading a short passage, about 20 seconds, and agree to it being kept. We
        keep the recording on our server, with the date you agreed. It is used only to speak your own words in your
        voice: when someone plays a message you sent, and when you use the interpreter. Fish Audio makes the speech from
        it for us. It is never given to other people.
      </p>
      <p>
        Some laws treat a recording of your voice like a fingerprint. That is why it is kept only with your agreement.
        You can take your agreement back at any time: in Profile, under Your voice, choose Delete my voice. It is
        deleted from our server and your phone, and your messages use the phone&apos;s voice instead.
      </p>

      <h2>Your messages</h2>
      <p>
        What you record or type in a room, the words written down from a recording, their translation, and the
        translation spoken in your voice. Microsoft&apos;s Azure AI Translator translates the words for us. Messages are
        kept so the people in the room can read and play them, until you delete your account.
      </p>
      <p>
        When you report a message, we keep it with your report as you saw it: its words, their translation and the
        recording. That copy stays until we have looked into the report, even if the person who sent it deletes their
        account, and is then deleted. The person you report or block is not told.
      </p>
      <p>
        Messages travel encrypted between your phone and our server, but they are not end-to-end encrypted: our server
        reads the words in order to translate them.
      </p>

      <h2>Writing down your words</h2>
      <p>
        Your phone writes down what you say by itself when it can. When it can&apos;t, on an older phone or before its
        speech model has downloaded, the recording goes to our server, and Groq writes the words down for us.
      </p>

      <h2>The interpreter</h2>
      <p>
        The interpreter listens and translates on your phone, with Google&apos;s ML Kit, whose language packs download
        from Google. ML Kit sends Google information about how it is used and how it performs, not linked to you.
      </p>
      <p>
        When the interpreter speaks in your voice, the words go to our server, with a copy of your voice if it is not
        kept there. When your phone can&apos;t write down your words, the recording goes to our server. Copies of your
        voice and recordings are deleted when the work is done, and the speech made from them within an hour. The words
        can stay in the server&apos;s records, not linked to your account.
      </p>

      <h2>Notifications</h2>
      <p>
        When a message reaches you and its room is not open, our server tells your phone through Apple on iPhone, or
        Google&apos;s Firebase on Android: the sender&apos;s name, and for a voice note how long it is, never its words
        or its sound. Your phone&apos;s address for notifications is kept with your sign-in and forgotten when you sign
        out. You can turn notifications off in your phone&apos;s settings.
      </p>

      <h2>Your subscription</h2>
      <p>
        If you subscribe, Apple tells our server what you bought: the plan, when it started and when it ends, whether it
        renews, and Apple&apos;s ID for the purchase, so we know you may send. We never see your card or bank details:
        Apple keeps those. The record of a subscription stays while Apple keeps it going, even if you delete your
        account, so that whoever restores it next can use it.
      </p>

      <h2>The waitlist</h2>
      <p>
        On this website you can leave your email address, your first name if you like, and which phone you have. We use
        them only to invite you to {APP_NAME} and to tell you when it&apos;s out. To invite you on iPhone, we give your
        address to Apple, whose TestFlight sends the invitation. We never give it to anyone else. We keep it until{" "}
        {APP_NAME} is out and we&apos;ve told you, or until you ask to be taken off the list.
      </p>

      <h2>Testing with TestFlight</h2>
      <p>
        While you test on iPhone, Apple&apos;s TestFlight shows us your name, your email address, when you installed{" "}
        {APP_NAME}, how often you open it, and its crash reports. That is under Apple&apos;s terms for TestFlight; we use
        it only to run the test.
      </p>

      <h2>This website</h2>
      <p>
        This website sets no cookies and has no analytics. Its fonts are served from the site itself. Vercel hosts it,
        and sees each visit as any host does: the address it comes from, and the browser. The waitlist form goes through
        Vercel to our server.
      </p>

      <h2>Where it&apos;s kept, and who helps</h2>
      <p>
        Our server and its database are on Microsoft Azure, in South Africa. These services do parts of the work for us,
        each with only what that part needs:
      </p>
      <ul>
        <li>
          <strong>Fish Audio</strong>: speaking your words in your voice.
        </li>
        <li>
          <strong>Microsoft Azure AI Translator</strong>: translating messages.
        </li>
        <li>
          <strong>Groq</strong>: writing down a recording when your phone can&apos;t.
        </li>
        <li>
          <strong>Google ML Kit</strong>: the interpreter&apos;s translation, on your phone.
        </li>
        <li>
          <strong>Apple and Google</strong>: signing in, and notifications.
        </li>
        <li>
          <strong>Apple&apos;s TestFlight</strong>: invitations, and the versions you test.
        </li>
        <li>
          <strong>Vercel</strong>: hosting this website.
        </li>
      </ul>
      <p>Some of them work in other countries, including the United States.</p>

      <h2>How long we keep it</h2>
      <ul>
        <li>Your account, your voice and your messages: until you delete them, or your account.</li>
        <li>A reported message: with its report, until we have looked into it.</li>
        <li>A subscription&apos;s record: as the record of what was paid, after it ends too.</li>
        <li>Recordings and copies of your voice sent for the interpreter: until the work is done. Its speech: an hour.</li>
        <li>The waitlist: until {APP_NAME} is out and we&apos;ve told you, or until you ask.</li>
        <li>Deleted data can stay in the database&apos;s automatic backups for up to 35 days, until they are replaced.</li>
      </ul>

      <h2>Deleting, and your other choices</h2>
      <p>
        Change your name, ID, language or voice in Profile. Delete your voice in Profile, under Your voice. Delete your
        account in Profile: your profile, your voice and everything you said in your rooms are deleted with it, except a
        message someone reported, which stays with their report until we have looked into it. This can&apos;t be undone. If you can&apos;t open the app, write to <Contact /> with your ID, and we&apos;ll delete it
        for you.
      </p>
      <p>
        You can also ask us for a copy of what we keep about you, or to correct it: write to <Contact />. If you&apos;re
        unhappy with how we handle your data, you can complain to the data protection authority where you live.
      </p>

      <h2>Age</h2>
      <p>
        {APP_NAME} is for people {MINIMUM_AGE} and over. If you think someone younger has signed up, write to{" "}
        <Contact />, and we&apos;ll delete the account.
      </p>

      <h2>Changes</h2>
      <p>
        When this page changes, the date at the top changes too. If a change affects how your voice or your messages are
        used, we&apos;ll say so here and in a new version&apos;s notes before it takes effect.
      </p>
    </TextPage>
  );
}
