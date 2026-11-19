// The name is written once, here, until its trademark check is done (ADR 0021).
// The wordmark is always the name in lowercase.
export const APP_NAME = "Shinar";
export const WORDMARK = APP_NAME.toLowerCase();
export const NAME_STORY = `${APP_NAME} is the plain in Genesis 11 where everyone still shared one language.`;

export const PROMISE = "Talk to anyone. Be heard in your own voice.";
export const PROMISE_LINE = "You speak your language. They hear theirs, in your voice.";

// The words of the example, from the design (docs/design/pearl, conversation
// and voice-note states): Rosa in Madrid, and her grandson in Lagos.
export const ROSA = {
  spanish: "A tu abuelo le habría encantado esa foto. Ven en agosto, el jardín estará lleno.",
  englishPhrases: ["Your grandfather would have loved that photo.", "Come in August,", "the garden will be full."],
  spanishShort: "Ven en agosto, el jardín estará lleno.",
  englishShort: "Come in August, the garden will be full.",
  duration: "0:14",
};

export type Stage = {
  title: string;
  body: string;
  time: string;
};

// A voice note's journey, as the app does it today (brief round 4, section 3).
export const STAGES: Stage[] = [
  {
    title: "She speaks",
    body: "Rosa taps the microphone and talks, in Spanish, for up to 30 seconds.",
    time: "As long as she talks",
  },
  {
    title: "Her words are written down",
    body: `Her phone writes down what she said, word by word. When it can't, ${APP_NAME}'s server does it.`,
    time: "About a second",
  },
  {
    title: "The note is sent",
    body: `The recording, her words and her language go to ${APP_NAME}'s server.`,
    time: "Under a second",
  },
  {
    title: "It is translated",
    body: "Her words are put into English, the language her grandson speaks.",
    time: "Under a second",
  },
  {
    title: "Her voice says it in English",
    body: "The English words are spoken in Rosa's own voice, made from the recording she gave when she joined and agreed to keep.",
    time: "2 to 5 seconds, the longest wait",
  },
  {
    title: "It reaches him",
    body: "His phone is told straight away. If he has their conversation open, the note appears; if not, he gets a notification.",
    time: "About a second",
  },
  {
    title: "He hears her",
    body: "He plays it: English, in his grandmother's voice. The words light up as she speaks, and her Spanish is a tap away.",
    time: "As long as the note",
  },
];

// Whose screen each stage happens on, by stage index.
export function stagePlace(stage: number): string {
  if (stage <= 2) return "On Rosa's phone, in Madrid";
  if (stage <= 4) return "On the way";
  return "On her grandson's phone, in Lagos";
}

// The sample in the hero plays these two files, one after the other, when both
// are in web/public/sample/ (see web/README.md). Without them it is left out.
export const SAMPLE = {
  original: "/sample/original.mp3",
  translated: "/sample/translated.mp3",
};

// Where people write to have their email address taken off the waitlist, and
// with any other question about what is kept. The owner's to set before the
// site goes live; until then the waitlist's note asks them to reply to the
// invitation instead.
export const CONTACT_EMAIL = "";

// The waitlist (ADR 0030). Testing is on iPhone only, through TestFlight;
// Android sign-ups are kept for when there is an Android test.
export const WAITLIST = {
  title: "Try it first",
  line: `${APP_NAME} is being tested on iPhone. Leave your email, and we'll send you an invitation through TestFlight, Apple's app for trying apps before they're out.`,
  android: "On Android? Leave your email too, and we'll write when it's ready for Android.",
  joinedIPhone: "You're on the list. Your invitation will come from TestFlight, by email.",
  joinedAndroid: `You're on the list. We'll write when ${APP_NAME} is ready for Android.`,
  privacy: `We use your email only to invite you to ${APP_NAME} and to tell you when it's out. To invite you on iPhone, we give it to Apple, whose TestFlight sends the invitation. We never give it to anyone else.`,
  // Posted to the name that served the page: Vercel passes it on to Shinar's
  // server (web/vercel.json), so the page never needs to know the server's
  // address.
  path: "/v1/waitlist",
};
