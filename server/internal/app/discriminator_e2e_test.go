//go:build e2e

package app_test

import (
	"net/http"
	"testing"

	"github.com/esuEdu/go-tauri-discord/pkg/events"
)

func (h *harness) registerAs(name, email string, want int) events.User {
	h.t.Helper()
	var out struct {
		User   events.User `json:"user"`
		Tokens struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	h.mustDo(http.MethodPost, "/api/v1/auth/register", want, map[string]string{
		"username": name,
		"email":    email,
		"password": "supersecret1",
	}, &out)
	if want == http.StatusCreated {
		h.token = out.Tokens.AccessToken
		h.email = email
	}
	return out.User
}

func TestTwoPeopleCanShareAUsername(t *testing.T) {
	first := newHarness(t)
	name := "Carla" + randomSuffix()

	one := first.registerAs(name, name+"-one@example.test", http.StatusCreated)

	second := &harness{t: t, server: first.server}
	two := second.registerAs(name, name+"-two@example.test", http.StatusCreated)

	if one.Username != two.Username {
		t.Fatalf("names differ: %q and %q", one.Username, two.Username)
	}
	if one.Discriminator == "" || two.Discriminator == "" {
		t.Fatal("somebody was registered with no number at all, so two people with one name " +
			"cannot be told apart anywhere")
	}
	if one.Discriminator == two.Discriminator {
		t.Fatalf("both got #%s; the pair that is supposed to be unique is not",
			one.Discriminator)
	}
	if one.Discriminator == "0000" || two.Discriminator == "0000" {
		t.Error("0000 was handed to a person; it belongs to the placeholder that deleted " +
			"accounts are reassigned to")
	}
}

func TestAnEmailStillCannotBeUsedTwice(t *testing.T) {
	h := newHarness(t)
	name := "Twice" + randomSuffix()
	email := name + "@example.test"

	h.registerAs(name, email, http.StatusCreated)

	other := &harness{t: t, server: h.server}
	other.registerAs(name+"-different", email, http.StatusConflict)
}

func TestTheNumberTravelsWithEveryMentionOfSomebody(t *testing.T) {
	owner := newHarness(t)
	owner.registerUser()
	guild := owner.createGuild("Tagged")
	text, _ := owner.textAndVoice(guild.ID)
	member := owner.inviteMember(guild.ID)

	sock := member.dial()
	ready := sock.identify(member.token)

	if ready.User.Discriminator == "" {
		t.Error("READY told a client its own number was empty")
	}
	for _, m := range ready.Members {
		if m.User.Discriminator == "" {
			t.Errorf("member %s arrived with no number, so the roster cannot disambiguate two "+
				"people with one name", m.User.Username)
		}
	}

	var posted events.Message
	owner.mustDo(http.MethodPost, "/api/v1/channels/"+text.String()+"/messages",
		http.StatusCreated, map[string]string{"content": "hello from a tagged author"}, &posted)
	if posted.Author.Discriminator == "" {
		t.Error("a message author carried no number, so two people with one name are " +
			"indistinguishable in the one place it matters most")
	}
}
