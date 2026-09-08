package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/guild"
)

func found(name string, members int64) invitePreview {
	return func(context.Context, string) (guild.InviteView, error) {
		return guild.InviteView{GuildName: name, MemberCount: members}, nil
	}
}

func unusable() invitePreview {
	return func(context.Context, string) (guild.InviteView, error) {
		return guild.InviteView{}, domain.NotFound("invite")
	}
}

func servePage(t *testing.T, path string, webClient bool, preview invitePreview) *httptest.ResponseRecorder {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /invite/{code}", invitePage(webClient, preview))

	out := httptest.NewRecorder()
	mux.ServeHTTP(out, httptest.NewRequest(http.MethodGet, path, nil))
	return out
}

func TestInvitePageReachesForTheAppThenTheDownload(t *testing.T) {
	out := servePage(t, "/invite/LBJJqars", true, found("Kitchen Table", 4))

	if out.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", out.Code)
	}
	if got := out.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
		t.Fatalf("want an HTML page, got %q", got)
	}

	body := out.Body.String()
	for _, want := range []string{"vocalis://invite/LBJJqars", DownloadURL, "/?invite=LBJJqars"} {
		if !strings.Contains(body, want) {
			t.Errorf("page never mentions %q", want)
		}
	}
}

func TestInvitePageNamesTheServer(t *testing.T) {
	body := servePage(t, "/invite/LBJJqars", true, found("Kitchen Table", 4)).Body.String()

	for _, want := range []string{"Kitchen Table", "4 members"} {
		if !strings.Contains(body, want) {
			t.Errorf("page never mentions %q", want)
		}
	}
	if !strings.Contains(body, "<title>Join Kitchen Table on Vocalis</title>") {
		t.Error("the tab does not say what the invite is for")
	}
}

func TestInvitePageCountsOneMemberSingly(t *testing.T) {
	body := servePage(t, "/invite/LBJJqars", true, found("Kitchen Table", 1)).Body.String()

	if !strings.Contains(body, "1 member ") {
		t.Error("page does not say 1 member")
	}
	if strings.Contains(body, "1 members") {
		t.Error("page says 1 members")
	}
}

func TestInvitePageWritesTheNameAsText(t *testing.T) {
	body := servePage(t, "/invite/LBJJqars", true, found(`<script>alert("x")</script>`, 2)).Body.String()

	if strings.Contains(body, "<script>alert") {
		t.Error("a server name reached the page as markup")
	}
}

func TestInvitePageSaysWhenAnInviteIsSpent(t *testing.T) {
	out := servePage(t, "/invite/LBJJqars", true, unusable())

	if out.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", out.Code)
	}

	body := out.Body.String()
	if !strings.Contains(body, "run out") {
		t.Error("page does not say the invite is spent")
	}
	if strings.Contains(body, "vocalis://invite/") {
		t.Error("page still reaches for the app with a dead invite")
	}
	if strings.Contains(body, "setTimeout") {
		t.Error("page still walks the visitor to the download")
	}
}

func TestInvitePageStillOpensTheAppWhenTheLookupBreaks(t *testing.T) {
	broken := func(context.Context, string) (guild.InviteView, error) {
		return guild.InviteView{}, errors.New("database is down")
	}
	out := servePage(t, "/invite/LBJJqars", true, broken)

	if out.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", out.Code)
	}
	if !strings.Contains(out.Body.String(), "vocalis://invite/LBJJqars") {
		t.Error("a broken lookup stopped the app from being offered")
	}
}

func TestInvitePageOffersTheBrowserOnlyWhenTheWebClientIsServed(t *testing.T) {
	body := servePage(t, "/invite/LBJJqars", false, found("Kitchen Table", 4)).Body.String()

	if strings.Contains(body, "?invite=LBJJqars") {
		t.Error("page offers a web client this server does not serve")
	}
	if !strings.Contains(body, "vocalis://invite/LBJJqars") {
		t.Error("page never reaches for the app")
	}
}

func TestInvitePageTurnsAwayCodesThatCouldNotBeOurs(t *testing.T) {
	looked := false
	watching := func(context.Context, string) (guild.InviteView, error) {
		looked = true
		return guild.InviteView{}, domain.NotFound("invite")
	}

	for _, path := range []string{"/invite/", "/invite/a%20b", `/invite/%22onload%3D`} {
		out := servePage(t, path, true, watching)
		if out.Code == http.StatusOK {
			t.Errorf("%s was served a page: %s", path, out.Body.String())
		}
	}
	if looked {
		t.Error("a code that could not be ours reached the database")
	}
}
