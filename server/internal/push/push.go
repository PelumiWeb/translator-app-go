// Package push sends notifications to phones: through Apple's push service
// (APNs) to iPhones, and through Firebase Cloud Messaging (FCM) to Android
// phones (ADR 0026). Both are called directly over HTTPS with the standard
// library; neither vendor's SDK is needed for sending.
package push

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/PelumiWeb/translator-app-go/server/internal/provider"
)

// The push services a phone can be reached through, as the phones name them
// and push_tokens.service stores them.
const (
	ServiceAPNs        = "apns"
	ServiceAPNsSandbox = "apns-sandbox" // Apple's test service, for builds run from Xcode
	ServiceFCM         = "fcm"
)

// ErrGone means the push service no longer knows the phone's address: the
// app was deleted, or the address was replaced. The address should be
// forgotten.
var ErrGone = errors.New("push: the address no longer reaches the app")

// Device is one app on one phone, as the push service knows it.
type Device struct {
	Service string // ServiceAPNs, ServiceAPNsSandbox or ServiceFCM
	// Token is Apple's device token, or the app's Firebase installation ID.
	Token string
}

// Notification is what to show, and what the app needs when it is tapped.
type Notification struct {
	Title string
	Body  string
	// Thread groups notifications on an iPhone, such as those of one room.
	Thread string
	// CollapseID makes a notification sent twice replace the first instead
	// of showing twice: a retried job sends again to every phone.
	CollapseID string
	// Data travels with the notification to the app: plain strings, since
	// FCM accepts nothing else.
	Data map[string]string
}

// Services sends each notification through the service its phone uses. A nil
// service is one this server is not set up for; phones that use it are skipped.
type Services struct {
	APNs *APNs
	FCM  *FCM
}

// Enabled reports whether any service is set up.
func (s Services) Enabled() bool { return s.APNs != nil || s.FCM != nil }

// Push sends n to one phone. Errors from the services say whether to retry:
// ErrGone, an error marked provider.Permanent, or anything else, which is
// worth trying again.
func (s Services) Push(ctx context.Context, to Device, n Notification) error {
	switch to.Service {
	case ServiceAPNs, ServiceAPNsSandbox:
		if s.APNs == nil {
			return nil
		}
		return s.APNs.Send(ctx, to.Token, to.Service == ServiceAPNsSandbox, n)
	case ServiceFCM:
		if s.FCM == nil {
			return nil
		}
		return s.FCM.Send(ctx, to.Token, n)
	default:
		return provider.Permanent(fmt.Errorf("push: unknown service %q", to.Service))
	}
}

// b64 is the encoding of every part of a JSON Web Token.
var b64 = base64.RawURLEncoding
