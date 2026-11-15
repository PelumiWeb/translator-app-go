package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestPutPushKeepsThePhonesAddress(t *testing.T) {
	apnsToken := strings.Repeat("ab12", 16)
	fcmToken := "dGVzdA:APA91bH-example_token.value"
	tests := []struct {
		name       string
		token      string // session; empty for none
		body       string
		wantStatus int
		wantKept   string
	}{
		{"an iPhone", "valid", `{"service":"apns","token":"` + apnsToken + `"}`, http.StatusNoContent, "apns " + apnsToken},
		{"an iPhone run from Xcode", "valid", `{"service":"apns-sandbox","token":"` + apnsToken + `"}`, http.StatusNoContent, "apns-sandbox " + apnsToken},
		{"an Android phone", "valid", `{"service":"fcm","token":"` + fcmToken + `"}`, http.StatusNoContent, "fcm " + fcmToken},
		{"an unknown service", "valid", `{"service":"pager","token":"` + apnsToken + `"}`, http.StatusBadRequest, ""},
		{"an APNs token that is not hex", "valid", `{"service":"apns","token":"../../3/device/x` + apnsToken + `"}`, http.StatusBadRequest, ""},
		{"an empty token", "valid", `{"service":"fcm","token":""}`, http.StatusBadRequest, ""},
		{"not JSON", "valid", `service=fcm`, http.StatusBadRequest, ""},
		{"signed out", "", `{"service":"fcm","token":"` + fcmToken + `"}`, http.StatusUnauthorized, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, session := signedInAPI()
			if tt.token == "" {
				session = ""
			}
			rec := send(a, http.MethodPut, "/v1/sessions/current/push", session, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, body %s; want %d", rec.Code, rec.Body, tt.wantStatus)
			}
			kept := a.Accounts.(*fakeAccounts).pushes[session]
			if kept != tt.wantKept {
				t.Errorf("kept %q, want %q", kept, tt.wantKept)
			}
		})
	}
}
