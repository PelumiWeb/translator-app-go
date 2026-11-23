package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/billing"
	"github.com/PelumiWeb/translator-app-go/server/internal/rooms"
)

// Paying for Shinar (milestone 21, ADR 0034).

// Billing is what the handlers need to know who has paid.
type Billing interface {
	Access(ctx context.Context, userID string) (billing.Access, error)
	AcceptPurchase(ctx context.Context, userID, signedTransaction, signedRenewalInfo string) (billing.Access, error)
	HandleNotification(ctx context.Context, signedPayload string) (billing.Notification, error)
}

type subscriptionResponse struct {
	// Required is false while the server lets everyone send (BILLING off):
	// the phone then shows no paywall.
	Required bool           `json:"required"`
	State    string         `json:"state"`
	Until    *time.Time     `json:"until,omitempty"`
	Renews   bool           `json:"renews"`
	Trial    *trialResponse `json:"trial,omitempty"`
}

// trialResponse is what is left of the free trial's cap.
type trialResponse struct {
	VoiceMSLeft int `json:"voice_ms_left"`
	TextsLeft   int `json:"texts_left"`
}

// handleGetSubscription says where the user stands: none, trial, active,
// grace or expired, until when, and in the trial, what is left of its cap.
func (a *API) handleGetSubscription(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	if a.Billing == nil {
		writeJSON(w, http.StatusOK, subscriptionResponse{State: string(billing.StateNone)})
		return
	}
	access, err := a.Billing.Access(r.Context(), user.ID)
	if err != nil {
		a.Logger.Error("reading access", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not read the subscription")
		return
	}
	a.writeSubscription(w, r, user, access)
}

// handleApplePurchase takes a purchase from the phone: the "signed_transaction"
// StoreKit gave it, after buying, restoring or opening the app, and its
// "signed_renewal_info" when it has one. Both are checked against Apple's
// signature before anything in them is believed.
func (a *API) handleApplePurchase(w http.ResponseWriter, r *http.Request, user accounts.User, _ string) {
	if a.Billing == nil {
		writeError(w, http.StatusNotImplemented, "billing_not_configured", "this server does not take purchases")
		return
	}
	var body struct {
		SignedTransaction string `json:"signed_transaction"`
		SignedRenewalInfo string `json:"signed_renewal_info"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	access, err := a.Billing.AcceptPurchase(r.Context(), user.ID, body.SignedTransaction, body.SignedRenewalInfo)
	switch {
	case errors.Is(err, billing.ErrInvalid), errors.Is(err, billing.ErrWrongApp):
		a.Logger.Warn("a purchase that did not verify", "user_id", user.ID, "error", err)
		writeError(w, http.StatusBadRequest, "invalid_purchase", "that purchase could not be checked with Apple")
		return
	case errors.Is(err, billing.ErrNotYours):
		writeError(w, http.StatusForbidden, "not_your_purchase", "that purchase was made for another account")
		return
	case errors.Is(err, billing.ErrOtherAccount):
		writeError(w, http.StatusConflict, "subscription_in_use", "this subscription belongs to another Shinar account")
		return
	case err != nil:
		a.Logger.Error("recording a purchase", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not record the purchase")
		return
	}
	a.Logger.Info("purchase recorded", "user_id", user.ID, "state", access.State)
	a.writeSubscription(w, r, user, access)
}

// handleAppleNotification takes one of the App Store's server notifications
// (version 2), sent by Apple with no sign-in: its signature is its proof. A
// payload that does not verify is answered 400; anything else that goes wrong,
// 500, which Apple sends again later.
func (a *API) handleAppleNotification(w http.ResponseWriter, r *http.Request) {
	if a.Billing == nil {
		writeError(w, http.StatusNotImplemented, "billing_not_configured", "this server does not take purchases")
		return
	}
	var body struct {
		SignedPayload string `json:"signedPayload"` // Apple's name for it, in Apple's case
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	n, err := a.Billing.HandleNotification(r.Context(), body.SignedPayload)
	switch {
	case errors.Is(err, billing.ErrInvalid), errors.Is(err, billing.ErrWrongApp):
		a.Logger.Warn("an App Store notification that did not verify", "error", err)
		writeError(w, http.StatusBadRequest, "invalid_notification", "the notification could not be checked")
		return
	case err != nil:
		a.Logger.Error("applying an App Store notification", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not apply the notification")
		return
	}
	a.Logger.Info("App Store notification", "type", n.Type, "subtype", n.Subtype, "id", n.UUID, "environment", n.Data.Environment)
	writeJSON(w, http.StatusOK, struct{}{})
}

func (a *API) writeSubscription(w http.ResponseWriter, r *http.Request, user accounts.User, access billing.Access) {
	response := subscriptionResponse{Required: a.RequireSubscription, State: string(access.State), Renews: access.Renews}
	if !access.Until.IsZero() {
		response.Until = &access.Until
	}
	if access.State == billing.StateTrial {
		voiceMS, texts, err := a.Rooms.SentSince(r.Context(), user.ID, access.Since)
		if err != nil {
			a.Logger.Error("adding up the trial", "user_id", user.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal", "could not read the subscription")
			return
		}
		response.Trial = &trialResponse{
			VoiceMSLeft: max(a.TrialLimits.VoiceMS-voiceMS, 0),
			TextsLeft:   max(a.TrialLimits.Texts-texts, 0),
		}
	}
	writeJSON(w, http.StatusOK, response)
}

// mayUse answers the request itself when the user may not send, because the
// server requires a subscription and they have none, and reports whether
// they may. In the free trial it returns the trial's allowance, for the
// rooms store to count against.
func (a *API) mayUse(w http.ResponseWriter, r *http.Request, user accounts.User) (trial *rooms.Allowance, ok bool) {
	if !a.RequireSubscription {
		return nil, true
	}
	access, err := a.Billing.Access(r.Context(), user.ID)
	if err != nil {
		a.Logger.Error("reading access", "user_id", user.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not check the subscription")
		return nil, false
	}
	if !access.CanSend() {
		// 402, which the phones do not retry: the answer is the paywall.
		writeError(w, http.StatusPaymentRequired, "subscription_required", "start the free trial, or subscribe, to send")
		return nil, false
	}
	if access.State == billing.StateTrial {
		return &rooms.Allowance{Since: access.Since, VoiceMS: a.TrialLimits.VoiceMS, Texts: a.TrialLimits.Texts}, true
	}
	return nil, true
}
