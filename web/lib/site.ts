// The name is written once, here, until its trademark check is done (ADR 0021).
// The wordmark is always the name in lowercase.
export const APP_NAME = "Shinar";
export const WORDMARK = APP_NAME.toLowerCase();
export const NAME_STORY = `${APP_NAME} is named for the plain in Genesis 11 where everyone still shared one language, before Babel and the language barrier.`;

// The theme of everything Shinar says is breaking the language barrier
// (ADR 0033). The promise and its sub-line are the same words as the apps'
// Welcome screen; change them together.
export const PROMISE = "Break the language barrier. Be heard in your own voice.";
export const PROMISE_LINE = "Talk to anyone. You speak your language. They hear theirs, in your voice.";

// Under the hero's buttons: what trying it costs and takes, before anyone
// scrolls to the form.
export const HERO_NOTE = "Free while we test. On iPhone, by invitation.";

// The site's address until there is a domain (the marketing brief keeps it in
// one place too). Link previews need it: they fetch the preview image by its
// full address.
export const SITE_URL = "https://web-nine-phi-84.vercel.app";

export type Example = { lead: string; rest: string };

// Who it's for: people a language apart who already know each other. Only
// Rosa's pair names languages, since only Spanish and English have been heard
// in her voice (design brief round 4, section 4).
export const WHO = {
  title: "Made for people a language apart",
  examples: [
    {
      lead: "A grandmother and her grandson.",
      rest: "She speaks Spanish; he grew up speaking English. Her notes reach him in his language, in her voice.",
    },
    {
      lead: "Your partner's family.",
      rest: "Say what you mean in your language. They hear it in theirs, in your voice.",
    },
    {
      lead: "The friend who moved home.",
      rest: "Keep talking, each in your own language. A voice note waits until they're free.",
    },
  ] satisfies Example[],
  line: "You add people by their ID or their code, and nobody can find you without one.",
};

export type Way = { title: string; text: string };

// What a tester can do today (marketing brief, section 4).
export const WAYS = {
  title: "Three ways to talk",
  line: "In a room of two, wherever each of you is, or face to face.",
  voice: {
    title: "Voice notes",
    text: "Speak for up to 30 seconds. They hear it in their language, in your voice, whenever they're free.",
  },
  text: {
    title: "Text messages",
    text: "When you'd rather type. They read it in their language, and can tap to hear it in your voice.",
  },
  interpreter: {
    title: "The interpreter",
    text: "Face to face, on one phone. Each of you speaks your own language, and the phone says it in the other's.",
  },
} satisfies Record<string, Way | string>;

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

// Where people write to have their email address taken off the waitlist, to
// ask about what is kept, or to report someone. When it is empty, the
// waitlist's note asks them to reply to an email from us instead.
export const CONTACT_EMAIL = "Pelumiogundipe905@gmail.com";

// The waitlist (ADR 0030). Testing is on iPhone only, through TestFlight;
// Android sign-ups are kept for when there is an Android test.
export const WAITLIST = {
  title: "Try it first",
  line: `Join with someone you don't share a language with. ${APP_NAME} is being tested on iPhone. Leave your email, and we'll send you an invitation through TestFlight, Apple's app for trying apps before they're out.`,
  android: "On Android? Leave your email too, and we'll write when it's ready for Android.",
  joinedIPhone: "You're on the list. Your invitation will come from TestFlight, by email.",
  joinedAndroid: `You're on the list. We'll write when ${APP_NAME} is ready for Android.`,
  // How joining goes, above the form: the pair comes first, since a room
  // needs both people.
  steps: [
    "Join the list with the person you want to talk to.",
    `Your invitation comes by email, from TestFlight. Install ${APP_NAME} from it.`,
    "Record your voice, add them by their ID, and send your first note.",
  ],
  privacy: `We use your email only to invite you to ${APP_NAME} and to tell you when it's out. To invite you on iPhone, we give it to Apple, whose TestFlight sends the invitation. We never give it to anyone else.`,
  // Posted to the name that served the page: Vercel passes it on to Shinar's
  // server (web/vercel.json), so the page never needs to know the server's
  // address.
  path: "/v1/waitlist",
};

// The help and legal pages say when they last changed. Change it with them.
export const PAGES_UPDATED = "19 November 2026";

// Whoever may use the app (ADR 0032).
export const MINIMUM_AGE = 16;

export type Question = { question: string; answer: string };

// The questions on the home page: what Shinar does, the tester FAQ of the
// soft launch, and two that visitors ask before they join. The answers match
// the app's privacy page; they make no promise it does not. Where an answer
// names CONTACT_EMAIL, the page makes it a link.
export const FAQ: Question[] = [
  {
    question: `What does ${APP_NAME} do?`,
    answer: `It breaks the language barrier between two people. You send a voice note in your language, and the other person hears it in theirs, in your own voice. They answer in their language, and you hear it in yours, in their voice. Text messages are translated too, and the interpreter helps two people talk face to face, on one phone.`,
  },
  {
    question: "Who can try it?",
    answer: `${APP_NAME} is being tested on iPhone, with iOS 18 or later: the iPhone XS and XR and everything newer. It's for people ${MINIMUM_AGE} and over. On Android? Leave your email too, and we'll write when it's ready for Android.`,
  },
  {
    question: "Does it cost anything?",
    answer: "No. It's free while we test.",
  },
  {
    question: "Why does the other person need it too?",
    answer: `${APP_NAME} turns your note into their language on its way to them, so they need ${APP_NAME} to hear it. You need it to hear their reply in yours. Join with the person you want to talk to.`,
  },
  {
    question: "Why does a note take a few seconds?",
    answer: `Each note is written down, translated, then spoken in your voice. That last step takes longest, usually 2 to 5 seconds. ${APP_NAME} is for voice notes, not calls.`,
  },
  {
    question: "Which languages work?",
    answer: `${APP_NAME} understands speech in many languages, but some work better than others. Yorùbá and Hausa are weaker, and Igbo isn't in yet. On the interpreter, two languages that aren't English are translated through English, so the result is plainer.`,
  },
  {
    question: "What happens to my voice?",
    answer: `Your recording is kept on ${APP_NAME}'s server with the date you agreed. It's used only to speak your own words when someone plays a message you sent, and it's never given to other people. Delete it any time in Profile, and your messages use the phone's voice. Deleting your account deletes your voice too.`,
  },
  {
    question: `Can ${APP_NAME} read my messages?`,
    answer: `Yes. Messages are not end-to-end encrypted: the server reads the words in order to translate them. They're kept so the people in the room can read and play them. TestFlight also shows us your name, email, install date, how often you open the app, and crash reports.`,
  },
  {
    question: "Why did I hear the phone's voice?",
    answer: "When the sender's voice can't be made, or they chose not to keep one, your phone reads the words aloud instead. The card says when this happens.",
  },
  {
    question: "I reached the limit",
    answer: `Each person can send 10 minutes of voice notes and 100 text messages in any 24 hours. Past that, your note waits on its card, and you can send it later. During testing, translation also runs on a small monthly allowance shared by every tester. If it runs out, translation stops for everyone until the next month. It's a test, and we're watching it.`,
  },
  {
    question: "How do I leave?",
    answer: `To delete your account and everything in it, go to Profile in ${APP_NAME} and choose Delete account. To stop testing, choose Stop Testing on ${APP_NAME}'s page in TestFlight. To come off the waitlist, ${CONTACT_EMAIL ? `write to ${CONTACT_EMAIL}` : "reply to any email from us"}.`,
  },
];
