import Foundation

/// What people read aloud when they record their voice: in their own language where there is a translation, in
/// English otherwise. About twenty seconds at an ordinary pace; varied sounds and a natural rhythm give a
/// voice-cloning engine more to work with than a list of words would. Split into the lines the teleprompter shows.
///
/// The words shown go to the server with the recording (`text(for:)`), which tells the voice engine what was said
/// in it. The English passage must stay word for word the same as on Android and on the server
/// (`provider.OnboardingPassage`), which assumes it for a recording that comes without its words.
enum VoicePassage {
    static func lines(for language: Language?) -> [String] {
        language.flatMap { translations[$0.tag] } ?? english
    }

    /// The lines as one text, as they were read.
    static func text(for language: Language?) -> String {
        // Chinese and Japanese put no spaces between words, so none between lines either.
        let separator = ["zh", "ja"].contains(language?.tag) ? "" : " "
        return lines(for: language).joined(separator: separator)
    }

    static let english = [
        "When I travel, I like to talk with the people I meet",
        "about their food, their music, and the places they love.",
        "A good conversation makes the world feel smaller.",
        "I am recording this so that when my words are translated,",
        "they can still be spoken in my own voice.",
    ]

    /// By Whisper's language tag, as profiles keep them. Each says what the English does, in the first person
    /// without saying whether the reader is a man or a woman, for the languages that would.
    static let translations: [String: [String]] = [
        "es": [
            "Cuando viajo, me gusta hablar con la gente que conozco",
            "sobre su comida, su música y los lugares que aman.",
            "Una buena conversación hace que el mundo parezca más pequeño.",
            "Grabo esto para que, cuando mis palabras se traduzcan,",
            "se sigan escuchando con mi propia voz.",
        ],
        "fr": [
            "Quand je voyage, j'aime parler avec les gens que je rencontre",
            "de leur cuisine, de leur musique et des endroits qu'ils aiment.",
            "Une bonne conversation rend le monde plus petit.",
            "J'enregistre ceci pour que, lorsque mes mots seront traduits,",
            "ils soient toujours dits avec ma propre voix.",
        ],
        "pt": [
            "Quando viajo, gosto de conversar com as pessoas que conheço",
            "sobre a comida, a música e os lugares que elas amam.",
            "Uma boa conversa faz o mundo parecer menor.",
            "Gravo isto para que, quando as minhas palavras forem traduzidas,",
            "continuem a ser ditas com a minha própria voz.",
        ],
        "de": [
            "Wenn ich reise, unterhalte ich mich gern mit den Menschen, die ich treffe,",
            "über ihr Essen, ihre Musik und die Orte, die sie lieben.",
            "Ein gutes Gespräch lässt die Welt kleiner wirken.",
            "Ich nehme das auf, damit meine Worte, wenn sie übersetzt werden,",
            "weiterhin mit meiner eigenen Stimme gesprochen werden.",
        ],
        "it": [
            "Quando viaggio, mi piace parlare con le persone che incontro",
            "del loro cibo, della loro musica e dei luoghi che amano.",
            "Una bella conversazione fa sembrare il mondo più piccolo.",
            "Registro queste parole perché, quando saranno tradotte,",
            "possano ancora essere dette con la mia voce.",
        ],
        "nl": [
            "Als ik op reis ben, praat ik graag met de mensen die ik ontmoet",
            "over hun eten, hun muziek en de plekken waar ze van houden.",
            "Een goed gesprek maakt de wereld kleiner.",
            "Ik neem dit op zodat mijn woorden, als ze vertaald worden,",
            "nog steeds met mijn eigen stem klinken.",
        ],
        "pl": [
            "Kiedy podróżuję, lubię rozmawiać z ludźmi, których spotykam,",
            "o ich jedzeniu, ich muzyce i miejscach, które kochają.",
            "Dobra rozmowa sprawia, że świat wydaje się mniejszy.",
            "Nagrywam to, aby moje słowa, kiedy zostaną przetłumaczone,",
            "wciąż brzmiały moim własnym głosem.",
        ],
        "ru": [
            "Когда я путешествую, я люблю разговаривать с людьми, которых встречаю,",
            "об их еде, их музыке и местах, которые они любят.",
            "Хороший разговор делает мир меньше.",
            "Я записываю это, чтобы мои слова, когда их переведут,",
            "всё равно звучали моим собственным голосом.",
        ],
        "uk": [
            "Коли я подорожую, я люблю розмовляти з людьми, яких зустрічаю,",
            "про їхню їжу, їхню музику та місця, які вони люблять.",
            "Добра розмова робить світ меншим.",
            "Я записую це, щоб мої слова, коли їх перекладуть,",
            "все одно звучали моїм власним голосом.",
        ],
        "tr": [
            "Seyahat ettiğimde tanıştığım insanlarla",
            "yemekleri, müzikleri ve sevdikleri yerler hakkında konuşmayı severim.",
            "İyi bir sohbet dünyayı daha küçük gösterir.",
            "Bunu, sözlerim çevrildiğinde de",
            "kendi sesimle söylenebilmeleri için kaydediyorum.",
        ],
        "ar": [
            "عندما أسافر، أحب أن أتحدث مع الناس الذين ألتقيهم",
            "عن طعامهم وموسيقاهم والأماكن التي يحبونها.",
            "المحادثة الجيدة تجعل العالم يبدو أصغر.",
            "أسجّل هذا لكي تبقى كلماتي، حين تُترجَم،",
            "تُقال بصوتي أنا.",
        ],
        "hi": [
            "यात्रा के दौरान मुझे उन लोगों से बात करना अच्छा लगता है जिनसे मेरी मुलाक़ात होती है,",
            "उनके खाने, उनके संगीत और उनकी पसंदीदा जगहों के बारे में।",
            "एक अच्छी बातचीत दुनिया को छोटा महसूस कराती है।",
            "यह रिकॉर्डिंग इसलिए है ताकि जब मेरे शब्दों का अनुवाद हो,",
            "तब भी वे मेरी अपनी आवाज़ में बोले जाएँ।",
        ],
        "zh": [
            "旅行的时候，我喜欢和遇到的人聊天，",
            "聊他们的美食、他们的音乐，还有他们喜爱的地方。",
            "一次愉快的交谈，会让世界变得更小。",
            "我录下这段话，是为了在我的话被翻译之后，",
            "仍然能用我自己的声音说出来。",
        ],
        "ja": [
            "旅をするとき、出会った人たちと",
            "その人たちの食べ物や音楽、大好きな場所について話すのが好きです。",
            "いい会話は、世界を小さく感じさせてくれます。",
            "これを録音するのは、私の言葉が翻訳されても、",
            "私自身の声で話されるようにするためです。",
        ],
        "ko": [
            "여행할 때 저는 만나는 사람들과",
            "그들의 음식과 음악, 그리고 그들이 사랑하는 곳에 대해 이야기하는 것을 좋아해요.",
            "좋은 대화는 세상을 더 작게 느끼게 해 줘요.",
            "제 말이 번역되더라도",
            "제 목소리로 들릴 수 있도록 이것을 녹음하고 있어요.",
        ],
        "id": [
            "Saat bepergian, saya suka mengobrol dengan orang-orang yang saya temui",
            "tentang makanan mereka, musik mereka, dan tempat-tempat yang mereka cintai.",
            "Percakapan yang baik membuat dunia terasa lebih kecil.",
            "Saya merekam ini agar saat kata-kata saya diterjemahkan,",
            "kata-kata itu tetap diucapkan dengan suara saya sendiri.",
        ],
        "vi": [
            "Khi đi du lịch, tôi thích trò chuyện với những người tôi gặp",
            "về món ăn, âm nhạc và những nơi họ yêu thích.",
            "Một cuộc trò chuyện hay khiến thế giới như nhỏ lại.",
            "Tôi ghi âm đoạn này để khi lời nói của tôi được dịch,",
            "chúng vẫn được nói bằng chính giọng của tôi.",
        ],
        "sw": [
            "Ninaposafiri, napenda kuzungumza na watu ninaokutana nao",
            "kuhusu vyakula vyao, muziki wao, na mahali wanapopapenda.",
            "Mazungumzo mazuri hufanya dunia ionekane ndogo.",
            "Ninarekodi hiki ili maneno yangu yanapotafsiriwa,",
            "bado yasemwe kwa sauti yangu mwenyewe.",
        ],
        "yo": [
            "Nígbà tí mo bá ń rìnrìn àjò, mo fẹ́ràn láti bá àwọn ènìyàn tí mo bá pàdé sọ̀rọ̀",
            "nípa oúnjẹ wọn, orin wọn, àti àwọn ibi tí wọ́n fẹ́ràn.",
            "Ìjíròrò tó dára máa ń jẹ́ kí ayé dà bí ẹni pé ó kéré sí i.",
            "Mo ń ká èyí sílẹ̀ kí ó lè jẹ́ pé, nígbà tí wọ́n bá túmọ̀ ọ̀rọ̀ mi,",
            "wọ́n ṣì lè sọ ọ́ ní ohùn ara mi.",
        ],
        "ha": [
            "Idan ina tafiya, ina son yin hira da mutanen da na haɗu da su",
            "game da abincinsu, waƙoƙinsu, da wuraren da suke so.",
            "Hira mai kyau takan sa duniya ta zama kamar ƙarama.",
            "Ina naɗar wannan ne domin idan an fassara kalmomina,",
            "a ci gaba da faɗar su da muryata.",
        ],
    ]
}
