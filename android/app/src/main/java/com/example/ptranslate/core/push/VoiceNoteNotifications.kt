package com.example.ptranslate.core.push

import android.Manifest
import android.app.PendingIntent
import android.content.Context
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.os.Build
import androidx.annotation.DrawableRes
import androidx.core.app.NotificationChannelCompat
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat

/**
 * Shows a notification for a voice note that arrives while its room is not on the screen, and takes it away when
 * the room opens. One per room: a newer note replaces the notification of the one before.
 *
 * @param openRoom what tapping the notification does: open the app on that room.
 */
class VoiceNoteNotifications(
    private val context: Context,
    @param:DrawableRes private val icon: Int,
    private val preferences: SharedPreferences,
    private val openRoom: (roomId: String) -> PendingIntent,
) {
    /** The room on the screen right now, if any. Its notes need no notification: they appear as they arrive. */
    @Volatile
    var roomOnScreen: String? = null

    /** Whether the app has asked for permission to notify. It asks once; after that, the phone's settings decide. */
    var permissionAsked: Boolean
        get() = preferences.getBoolean(PERMISSION_ASKED, false)
        set(value) = preferences.edit().putBoolean(PERMISSION_ASKED, value).apply()

    private val manager = NotificationManagerCompat.from(context)

    fun show(roomId: String, title: String, body: String) {
        if (roomId == roomOnScreen || !allowed()) return
        manager.createNotificationChannel(
            NotificationChannelCompat.Builder(CHANNEL, NotificationManagerCompat.IMPORTANCE_HIGH)
                .setName("Voice notes")
                .build(),
        )
        val notification = NotificationCompat.Builder(context, CHANNEL)
            .setSmallIcon(icon)
            .setContentTitle(title)
            .setContentText(body)
            .setCategory(NotificationCompat.CATEGORY_MESSAGE)
            .setAutoCancel(true)
            .setContentIntent(openRoom(roomId))
            .build()
        try {
            manager.notify(roomId, ID, notification)
        } catch (_: SecurityException) {
            // The permission was taken away between the check and now.
        }
    }

    fun cancel(roomId: String) = manager.cancel(roomId, ID)

    private fun allowed(): Boolean {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
        ) {
            return false
        }
        return manager.areNotificationsEnabled()
    }

    private companion object {
        const val CHANNEL = "voice_notes"

        // The room's ID is the notification's tag, which tells rooms apart; the number is the same for all.
        const val ID = 1
        const val PERMISSION_ASKED = "notification_permission_asked"
    }
}
