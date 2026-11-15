import Observation
import UserNotifications

/// Decides what happens to a voice note's notification (ADR 0026): it is not shown while its room is on the screen,
/// where the note appears as it arrives; tapping it opens the room; opening the room clears the room's notifications.
/// The server writes the words, and groups each room's notifications under its ID.
@MainActor
@Observable
final class VoiceNoteNotifications: NSObject, UNUserNotificationCenterDelegate {
    /// The room on the screen right now, if any.
    @ObservationIgnored var roomOnScreen: String?

    /// A room whose notification was tapped, waiting to be opened.
    private(set) var roomToOpen: String?

    @ObservationIgnored private let center: UNUserNotificationCenter

    init(center: UNUserNotificationCenter = .current()) {
        self.center = center
        super.init()
    }

    func roomOpened() {
        roomToOpen = nil
    }

    /// Asks once for permission to notify: the system's own question, as the screen before it is not designed yet.
    /// Later calls return the answer given, without asking again.
    func requestPermission() async -> Bool {
        (try? await center.requestAuthorization(options: [.alert, .sound])) ?? false
    }

    /// Takes away the notifications of `roomID`, now that it is open.
    func clear(roomID: String) async {
        let delivered = await center.deliveredNotifications()
        let ids = delivered.filter { $0.request.content.threadIdentifier == roomID }.map(\.request.identifier)
        center.removeDeliveredNotifications(withIdentifiers: ids)
    }

    nonisolated func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification
    ) async -> UNNotificationPresentationOptions {
        let room = Self.room(of: notification)
        let onScreen = await MainActor.run { roomOnScreen }
        return room != nil && room == onScreen ? [] : [.banner, .list, .sound]
    }

    nonisolated func userNotificationCenter(_ center: UNUserNotificationCenter, didReceive response: UNNotificationResponse) async {
        guard let room = Self.room(of: response.notification) else { return }
        await MainActor.run { roomToOpen = room }
    }

    private nonisolated static func room(of notification: UNNotification) -> String? {
        notification.request.content.userInfo["room_id"] as? String
    }
}
