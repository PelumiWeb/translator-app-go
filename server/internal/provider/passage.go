package provider

// OnboardingPassage is what the phones ask users to read when they record
// their voice, in English. The iPhone shows it in the person's own language
// when it has a translation (VoicePassage), and sends the words it showed with
// the sample; a sample that comes without them was read in English, so this
// is its transcript. Engines that imitate a voice from a sample and its
// transcript are given it. It must stay word for word the same as the English
// passage on both phones.
const OnboardingPassage = "When I travel, I like to talk with the people I meet about their food, their music, " +
	"and the places they love. A good conversation makes the world feel smaller. I am recording this so that " +
	"when my words are translated, they can still be spoken in my own voice."
