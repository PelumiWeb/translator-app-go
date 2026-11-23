package billing

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Service is billing as the API uses it: purchases the phones send, and the
// notifications Apple sends, both checked before anything is believed.
type Service struct {
	Verifier Verifier
	Store    *Store
	// Now is time.Now on a server; tests set the clock.
	Now func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

// Access is where userID stands now.
func (s *Service) Access(ctx context.Context, userID string) (Access, error) {
	return s.Store.Access(ctx, userID, s.now())
}

// AcceptPurchase applies a signed transaction the phone of userID sent, after
// a purchase, a restore or a launch, with its renewal information when the
// phone has it, and returns where the user now stands.
func (s *Service) AcceptPurchase(ctx context.Context, userID, signedTransaction, signedRenewalInfo string) (Access, error) {
	t, err := s.Verifier.Transaction(signedTransaction)
	if err != nil {
		return Access{}, err
	}
	// The phone buys with the user's ID as the token. A purchase made without
	// one (an offer code redeemed in the App Store, say) is taken by the
	// account that sends it.
	if t.AppAccountToken != "" && !strings.EqualFold(t.AppAccountToken, userID) {
		return Access{}, ErrNotYours
	}
	var renewal *RenewalInfo
	if signedRenewalInfo != "" {
		r, err := s.Verifier.RenewalInfo(signedRenewalInfo)
		if err != nil {
			return Access{}, err
		}
		if r.OriginalTransactionID != t.OriginalTransactionID {
			return Access{}, fmt.Errorf("%w: the renewal information is another subscription's", ErrInvalid)
		}
		renewal = &r
	}
	if err := s.Store.Record(ctx, userID, t, renewal); err != nil {
		return Access{}, err
	}
	return s.Access(ctx, userID)
}

// HandleNotification applies one of Apple's server notifications: a renewal,
// a failed payment, a refund, an expiry. It reads only the transaction and
// renewal information inside, each checked on its own, so every kind of
// notification is handled the same way and a kind Apple adds later needs
// nothing new. It returns the notification, for the log.
func (s *Service) HandleNotification(ctx context.Context, signedPayload string) (Notification, error) {
	n, err := s.Verifier.Notification(signedPayload)
	if err != nil {
		return Notification{}, err
	}
	if n.Data.SignedTransactionInfo != "" {
		t, err := s.Verifier.Transaction(n.Data.SignedTransactionInfo)
		if err != nil {
			return Notification{}, err
		}
		var renewal *RenewalInfo
		if n.Data.SignedRenewalInfo != "" {
			r, err := s.Verifier.RenewalInfo(n.Data.SignedRenewalInfo)
			if err != nil {
				return Notification{}, err
			}
			renewal = &r
		}
		if err := s.Store.Record(ctx, "", t, renewal); err != nil {
			return Notification{}, err
		}
	}
	// Recorded after it is applied: applying twice changes nothing, so a
	// notification whose recording failed is safe for Apple to send again.
	if err := s.Store.Noted(ctx, n, signedPayload); err != nil {
		return Notification{}, err
	}
	return n, nil
}
