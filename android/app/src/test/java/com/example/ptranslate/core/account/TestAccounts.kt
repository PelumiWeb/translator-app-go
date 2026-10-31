package com.example.ptranslate.core.account

import com.example.ptranslate.core.net.AccountClient
import com.example.ptranslate.core.net.Rest
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockWebServer

class MemorySessionStore(var session: Session? = null) : SessionStore {
    override fun load() = session
    override fun save(session: Session) {
        this.session = session
    }
    override fun clear() {
        session = null
    }
}

class MemoryProgress(override var step: OnboardingStep = OnboardingStep.NAME) : OnboardingProgress

/** An account talking to [server], or to nowhere when there is none: enough for a signed-out user. */
fun testAccount(
    server: MockWebServer? = null,
    store: MemorySessionStore = MemorySessionStore(),
    progress: MemoryProgress = MemoryProgress(),
): Account {
    lateinit var account: Account
    val rest = Rest(
        baseUrl = server?.url("/")?.toString() ?: "http://localhost:1/",
        http = OkHttpClient(),
        token = { account.token },
        onSessionEnded = { account.forget() },
    )
    account = Account(AccountClient(rest), store, progress)
    return account
}
