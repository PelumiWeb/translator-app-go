import SwiftUI

/// What is kept, and why, in plain words. The same text on Android.
struct PrivacyScreen: View {
    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                Text("What we keep, and why").font(.pearlScreenTitle).accessibilityAddTraits(.isHeader)
                ForEach(Self.sections, id: \.title) { section in
                    VStack(alignment: .leading, spacing: 4) {
                        Text(section.title).font(.pearlMessageStrong).foregroundStyle(Palette.fg)
                        Text(section.text).font(.pearlMessage).foregroundStyle(Palette.fg2)
                    }
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, 28)
            .padding(.vertical, 16)
        }
        .foregroundStyle(Palette.fg)
        .background(Palette.bg)
        .navigationBarTitleDisplayMode(.inline)
    }

    private static var sections: [(title: String, text: String)] {
        [
            (String(localized: "Your account"), String(localized: "Your name, your ID and the language you speak, so people can find you and hear you in their language. We keep your email address only if Apple has shared it, to recognise your account. We also keep a token Apple gives us, used only to end your Sign in with Apple when you delete your account, and when you agreed to the services that help, below.")),
            (String(localized: "Your voice"), String(localized: "The recording you made, on our server, with the date you agreed. It is used only to speak your own words when someone plays a message you sent. Fish Audio makes the speech for us. It is never given to other people.")),
            (String(localized: "Your messages"), String(localized: "What you record or type in a room, the words written down from a recording, their translation, and the translation spoken in your voice. Microsoft's Azure AI Translator translates the words. They are kept so the people in the room can read and play them.")),
            (String(localized: "Writing down your words"), String(localized: "Your phone writes down what you say by itself when it can. When it cannot, on an older phone or before its speech model has downloaded, the recording goes to our server, and Groq writes the words down for us.")),
            (String(localized: "Notifications"), String(localized: "When a message reaches you and its room is not open, our server tells your iPhone through Apple: the sender's name, and for a voice note how long it is, never its words or its sound. Turn them off in Settings, under \(Brand.name).")),
            (String(localized: "Your subscription"), String(localized: "If you subscribe, Apple tells our server what you bought: the plan, when it started and ends, whether it renews, and Apple's ID for the purchase, so we know you may send. We never see your card or bank details: Apple keeps those.")),
            (String(localized: "Reports and blocking"), String(localized: "When you report a message, we keep it with your report, as you saw it: its words, their translation and the recording, until we have looked into it, even if the person who sent it deletes their account. Then that copy is deleted. The person you report or block is not told.")),
            (String(localized: "Deleting"), String(localized: "Delete your voice in Profile, under Your voice: your messages then use the phone's voice. Delete your account, and your profile, your voice and everything you said in your rooms are deleted with it, except a message someone reported, which stays with the report until we have looked into it.")),
        ]
    }
}
