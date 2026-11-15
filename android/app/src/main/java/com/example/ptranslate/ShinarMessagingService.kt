package com.example.ptranslate

import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

/**
 * Firebase hands this service what the server sends through it (ADR 0026): a voice note ready in a room, and the
 * app's address, its Firebase installation ID. The server sends data messages, so this runs for each one, in the
 * foreground or not, and decides what to show.
 */
class ShinarMessagingService : FirebaseMessagingService() {
    private val container get() = (application as PtranslateApp).container

    override fun onRegistered(installationId: String) = container.push.onAddress(installationId)

    override fun onMessageReceived(message: RemoteMessage) {
        // Sent before a sign-out reached the server: not for whoever uses the phone now.
        if (container.account.session.value == null) return
        val data = message.data
        val room = data["room_id"] ?: return
        container.notifications.show(room, data["title"].orEmpty(), data["body"].orEmpty())
    }
}
