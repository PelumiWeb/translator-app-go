package rooms

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/PelumiWeb/translator-app-go/server/internal/accounts"
	"github.com/PelumiWeb/translator-app-go/server/internal/push"
)

// reportOf reports a message of rosa's as ada saw it, with Spanish
// translated into English.
func reportOf(ada, rosa string, m Message, block bool) NewReport {
	return NewReport{
		ReporterID: ada, ReportedID: rosa, Reason: ReasonHarassment, Note: "every day", Block: block,
		Message: &ReportedMessage{
			ID: m.ID, Kind: m.Kind, Lang: m.Lang, Transcript: m.Transcript,
			Translation: "you again", AudioKey: "copy.wav",
		},
	}
}

func TestAReportKeepsTheMessageAfterItsSenderIsGone(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	handle := "rosa.m"
	if _, err := w.accounts.UpdateProfile(ctx, rosa, accounts.ProfileUpdate{Handle: &handle}); err != nil {
		t.Fatal(err)
	}
	room := w.open(t, ada, rosa)
	m := w.send(t, room, rosa, "es", "tú otra vez")
	id, err := w.rooms.AddReport(ctx, reportOf(ada, rosa, m, true))
	if err != nil {
		t.Fatal(err)
	}

	// Rosa deletes her account, and with it everything she said.
	if _, err := w.rooms.ForgetSender(ctx, rosa); err != nil {
		t.Fatal(err)
	}
	if err := w.accounts.DeleteUser(ctx, rosa); err != nil {
		t.Fatal(err)
	}

	open, err := w.rooms.OpenReports(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].ID != id {
		t.Fatalf("open reports = %+v, want the one", open)
	}
	got := open[0]
	if got.ReportedID != "" || got.Reported != "@rosa.m rosa" {
		t.Errorf("reported = %q (id %q), want @rosa.m rosa as she was, with no account", got.Reported, got.ReportedID)
	}
	if got.ReporterID != ada || got.Reporter != "ada" || got.Reason != ReasonHarassment || got.Note != "every day" {
		t.Errorf("report = %+v", got)
	}
	want := ReportedMessage{ID: m.ID, Kind: KindVoice, Lang: "es", Transcript: "tú otra vez", Translation: "you again", AudioKey: "copy.wav"}
	if got.Message == nil || *got.Message != want {
		t.Errorf("message = %+v, want %+v", got.Message, want)
	}
}

func TestAReportBlocksUnlessAskedNotTo(t *testing.T) {
	for _, block := range []bool{true, false} {
		w := newWorld(t)
		ctx := context.Background()
		ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
		room := w.open(t, ada, rosa)
		m := w.send(t, room, rosa, "es", "hola")
		if _, err := w.rooms.AddReport(ctx, reportOf(ada, rosa, m, block)); err != nil {
			t.Fatal(err)
		}
		_, _, err := w.rooms.AddMessage(ctx, NewMessage{RoomID: room, SenderID: rosa, Kind: KindText, Lang: "es", Transcript: "hola"})
		if blocked := errors.Is(err, ErrBlocked); blocked != block {
			t.Errorf("block %v: sending afterwards gave %v", block, err)
		}
	}
}

func TestAReportAboutAPerson(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa, kenji := w.user(t, "ada", "en"), w.user(t, "rosa", "es"), w.user(t, "kenji", "ja")
	w.open(t, ada, rosa)

	tests := []struct {
		name     string
		reported string
		want     error
	}{
		{"someone in a room with them", rosa, nil},
		{"a stranger", kenji, ErrNoPerson},
		{"themselves", ada, ErrNoPerson},
		{"no one", "00000000-0000-0000-0000-000000000000", ErrNoPerson},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.rooms.AddReport(ctx, NewReport{ReporterID: ada, ReportedID: tt.reported, Reason: ReasonSpam})
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
	open, err := w.rooms.OpenReports(ctx)
	if err != nil || len(open) != 1 || open[0].Message != nil {
		t.Errorf("open = %+v (%v), want one, about a person, with no message", open, err)
	}
}

func TestAReportQueuesANoticeForTheOwner(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	w.open(t, ada, rosa)
	id, err := w.rooms.AddReport(ctx, NewReport{ReporterID: ada, ReportedID: rosa, Reason: ReasonSpam})
	if err != nil {
		t.Fatal(err)
	}
	var notices int
	if err := w.db.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE kind = 'report' AND report_id = $1`, id).Scan(&notices); err != nil {
		t.Fatal(err)
	}
	if notices != 1 {
		t.Errorf("notice jobs = %d, want 1", notices)
	}
}

func TestClosingAReportClearsItsCopy(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa := w.user(t, "ada", "en"), w.user(t, "rosa", "es")
	room := w.open(t, ada, rosa)
	m := w.send(t, room, rosa, "es", "hola")
	id, err := w.rooms.AddReport(ctx, reportOf(ada, rosa, m, true))
	if err != nil {
		t.Fatal(err)
	}

	key, err := w.rooms.CloseReport(ctx, id, "warned them")
	if err != nil || key != "copy.wav" {
		t.Fatalf("CloseReport = %q, %v; want the copy's key", key, err)
	}
	var transcript, translation, audio *string
	var outcome string
	if err := w.db.QueryRow(ctx, `SELECT transcript, translation, audio_key, outcome FROM reports WHERE id = $1`, id).Scan(&transcript, &translation, &audio, &outcome); err != nil {
		t.Fatal(err)
	}
	if transcript != nil || translation != nil || audio != nil || outcome != "warned them" {
		t.Errorf("after closing: %v %v %v %q, want the copy gone and the outcome kept", transcript, translation, audio, outcome)
	}
	if open, _ := w.rooms.OpenReports(ctx); len(open) != 0 {
		t.Errorf("open reports = %+v, want none", open)
	}
	for _, again := range []string{id, "00000000-0000-0000-0000-000000000000"} {
		if _, err := w.rooms.CloseReport(ctx, again, ""); !errors.Is(err, ErrNoReport) {
			t.Errorf("closing %s again: err = %v, want ErrNoReport", again, err)
		}
	}
}

func TestTheOwnerIsToldOfAReport(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	ada, rosa, owner := w.user(t, "ada", "en"), w.user(t, "rosa", "es"), w.user(t, "owner", "en")
	handle := "pelumi"
	if _, err := w.accounts.UpdateProfile(ctx, owner, accounts.ProfileUpdate{Handle: &handle}); err != nil {
		t.Fatal(err)
	}
	w.phone(t, "owner", push.ServiceAPNs, "owner-phone")
	w.phone(t, "ada", push.ServiceAPNs, "ada-phone")
	room := w.open(t, ada, rosa)
	m := w.send(t, room, rosa, "es", "hola")
	id, err := w.rooms.AddReport(ctx, reportOf(ada, rosa, m, true))
	if err != nil {
		t.Fatal(err)
	}

	pusher := &fakePusher{}
	d := &Deliverer{Store: w.rooms, Pusher: pusher, ReportsTo: "Pelumi"} // the ID's case does not matter
	if err := d.NotifyReport(ctx, id); err != nil {
		t.Fatal(err)
	}
	if want := []string{"apns owner-phone"}; !slices.Equal(pusher.sent, want) {
		t.Errorf("sent to %v, want %v: the owner's phone only", pusher.sent, want)
	}
	if pusher.last.Title != "New report" || pusher.last.Body != "A voice note · harassment" {
		t.Errorf("notification = %q / %q", pusher.last.Title, pusher.last.Body)
	}
	if pusher.last.Data["report_id"] != id || pusher.last.Data["room_id"] != "" {
		t.Errorf("data = %v, want the report and no room", pusher.last.Data)
	}

	// With no owner named, nobody is told, and the job is done.
	quiet := &fakePusher{}
	d = &Deliverer{Store: w.rooms, Pusher: quiet}
	if err := d.NotifyReport(ctx, id); err != nil || len(quiet.sent) != 0 {
		t.Errorf("with no owner: sent %v, err %v; want nothing", quiet.sent, err)
	}
}

func TestSpeakableLeavesOutMaskedWords(t *testing.T) {
	tests := []struct{ text, want string }{
		{"I said what I said", "I said what I said"},
		{"You are a *** and a ***.", "You are a and a."},
		{"*** you", "you"},
		{"***", ""},
		{"2 * 3 = 6", "2 * 3 = 6"},
	}
	for _, tt := range tests {
		if got := speakable(tt.text); got != tt.want {
			t.Errorf("speakable(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}
