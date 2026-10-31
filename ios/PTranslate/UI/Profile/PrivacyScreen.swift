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

    private static let sections: [(title: String, text: String)] = [
        ("Your account", "Your name, your ID and the language you speak, so people can find you and hear you in their language. "
            + "We keep your email address only if Apple has shared it, to recognise your account."),
        ("Your voice", "The recording you made, on our server, with the date you agreed. It is used only to speak your own words "
            + "when someone plays a message you sent. Fish Audio makes the speech for us. It is never given to other people."),
        ("Your messages", "What you record in a room, the words written down from it, its translation, and the translation spoken in "
            + "your voice. Google Cloud Translation translates the words. They are kept so the people in the room can play them."),
        ("Deleting", "Delete your voice in Profile, under Your voice: your messages then use the phone's voice. Delete your account, "
            + "and your profile, your voice and everything you said in your rooms are deleted with it."),
    ]
}
