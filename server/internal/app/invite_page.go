package app

import (
	"context"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/esuEdu/go-tauri-discord/internal/domain"
	"github.com/esuEdu/go-tauri-discord/internal/guild"
)

const DownloadURL = "https://github.com/esuEdu/go-tauri-discord/releases/latest"

var inviteCodeShape = regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`)

type invitePreview func(ctx context.Context, code string) (guild.InviteView, error)

type invitePageView struct {
	Guild    string
	Members  int64
	Gone     bool
	App      template.URL
	Browser  string
	Download string
}

var invitePageTemplate = template.Must(template.New("invite").Parse(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="robots" content="noindex" />
    <title>{{if .Guild}}Join {{.Guild}} on Vocalis{{else}}Vocalis{{end}}</title>
    <style>
      :root {
        color-scheme: dark;
        --ground: #0b0d16;
        --surface: #232532;
        --text: #e9e9ed;
        --quiet: #9397ab;
        --accent: #968ae0;
        --edge: #3f424d;
        --font: "Inter Variable", "Inter", system-ui, -apple-system, sans-serif;
      }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        min-height: 100vh;
        display: grid;
        place-items: center;
        padding: 24px;
        background: var(--ground);
        color: var(--text);
        font: 400 14px/1.55 var(--font);
      }
      main {
        width: 100%;
        max-width: 420px;
        padding: 32px;
        border-radius: 14px;
        background: var(--surface);
        box-shadow: 0 0 0 1px var(--edge), 0 16px 40px rgb(0 0 0 / 0.55);
        text-align: center;
      }
      .eyebrow {
        margin: 0 0 4px;
        color: var(--quiet);
        font: 500 11px/1 var(--font);
        letter-spacing: 0.08em;
        text-transform: uppercase;
      }
      h1 { margin: 0 0 8px; font: 500 20px/1.2 var(--font); overflow-wrap: anywhere; }
      p { margin: 0 0 24px; color: var(--quiet); }
      .row { display: grid; gap: 8px; }
      a.button {
        display: block;
        padding: 10px 16px;
        border-radius: 8px;
        background: var(--accent);
        color: #16121f;
        font: 500 14px/1.4 var(--font);
        text-decoration: none;
      }
      a.button.quiet {
        background: transparent;
        color: var(--text);
        box-shadow: inset 0 0 0 1px var(--edge);
      }
      a.plain {
        display: inline-block;
        margin-top: 20px;
        color: var(--quiet);
        font: 400 13px/1.5 var(--font);
      }
    </style>
  </head>
  <body>
    <main>
{{if .Gone}}      <h1>This invite has run out</h1>
      <p>It expired, was used up, or was taken back. Ask whoever sent it for a new one.</p>
      <div class="row">
        <a class="button quiet" href="{{.Download}}">Download Vocalis</a>
      </div>
{{else}}{{if .Guild}}      <p class="eyebrow">You have been invited to join</p>
      <h1>{{.Guild}}</h1>
      <p>{{.Members}} {{if eq .Members 1}}member{{else}}members{{end}} · opening Vocalis. If the app is not installed, the download page opens instead.</p>
{{else}}
      <h1>Opening Vocalis</h1>
      <p>Your invite is being handed to the app. If Vocalis is not installed, the download page opens instead.</p>{{end}}
      <div class="row">
        <a class="button" href="{{.App}}">Open Vocalis</a>
        <a class="button quiet" href="{{.Download}}">Download Vocalis</a>
      </div>
      {{if .Browser}}<a class="plain" href="{{.Browser}}">Continue in this browser</a>{{end}}{{end}}
    </main>
{{if not .Gone}}    <script>
      var app = "{{.App}}";
      var download = "{{.Download}}";
      var away = false;
      function gone() { away = true; }
      addEventListener("blur", gone);
      addEventListener("pagehide", gone);
      document.addEventListener("visibilitychange", function () {
        if (document.hidden) gone();
      });
      setTimeout(function () { location.href = app; }, 50);
      setTimeout(function () {
        if (away || document.hidden || !document.hasFocus()) return;
        location.replace(download);
      }, 2500);
    </script>{{end}}
  </body>
</html>
`))

func invitePage(webClient bool, preview invitePreview) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")
		if !inviteCodeShape.MatchString(code) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		page := invitePageView{
			App:      template.URL("vocalis://invite/" + code),
			Download: DownloadURL,
		}
		if webClient {
			page.Browser = "/?invite=" + code
		}

		status := http.StatusOK
		switch invite, err := preview(r.Context(), code); {
		case err == nil:
			page.Guild = invite.GuildName
			page.Members = invite.MemberCount
		case domain.KindOf(err) == domain.KindNotFound:
			page.Gone = true
			status = http.StatusNotFound
		default:
			slog.Error("invite page could not read the invite", "error", err)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = invitePageTemplate.Execute(w, page)
	}
}
