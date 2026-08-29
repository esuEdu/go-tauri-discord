# Deploying Vocalis

This is the runbook for putting Vocalis on a machine other people can reach.
It describes one supported shape — a single Linux host running Docker, with
Caddy terminating TLS in front of the server — because that is the shape voice
actually works in, and every deviation from it changes something about media.

For an evening with friends, skip all of this and run `make share`: it serves
the app and opens a Cloudflare tunnel. The difference is written down at the
end, under [Why not just the tunnel](#why-not-just-the-tunnel).

---

## What it needs

- **A Linux host with Docker and the compose plugin.** One machine. Vocalis
  keeps gateway fanout and every SFU track in memory, so a second replica would
  not see the first one's rooms — see *Scaling past one node* in the README.
- **A domain name** with an A record pointing at that host, resolving *before*
  you start Caddy. A certificate request that fails still counts against Let's
  Encrypt's rate limit.
- **The host's own public address.** If the address people reach is mapped to a
  private NIC — EC2, GCP, anything behind a NAT gateway — you must set
  `WEBRTC_PUBLIC_IP`. On a plain VPS you leave it empty.
- **These ports open:**

  | Port | Protocol | For |
  |------|----------|-----|
  | 80 | TCP | Caddy's certificate challenge and the redirect to 443 |
  | 443 | TCP | The app, the websocket gateway, uploads |
  | 50000–50999 | **UDP** | WebRTC media: voice and screen sharing |

  The UDP range is the one people forget. TCP 443 alone gets you a chat app
  where nobody can hear anybody.

---

## Deploying

```bash
git clone https://github.com/esuEdu/go-tauri-discord.git
cd go-tauri-discord

make deploy-env          # writes .env with a generated password and JWT_SECRET
$EDITOR .env             # set DOMAIN, CORS_ORIGINS, and WEBRTC_PUBLIC_IP if needed
make deploy-up           # builds the image, starts Postgres, the server and Caddy
```

`make deploy-up` refuses to run against a development `.env`, because that file
points at a local database with a password committed to this public repository.

Then watch it come up:

```bash
make deploy-logs
```

A healthy first boot says `database connected`, one `migration applied` line
per migration, and `listening`. If it says *voice will not connect: this host
has no public address*, stop and set `WEBRTC_PUBLIC_IP` — everything except
media will work, which is the confusing way for that to fail.

Open `https://your-domain`, register, and the first account is just an account:
there is no admin, and whoever creates a server owns it.

---

## Oracle Cloud, and a name without buying one

Oracle's free tier is a good host for this and gets two things wrong by
default. Both fail in ways that look like the application is broken.

**Its firewall is two firewalls.** Opening ports in the VCN security list is
only half: Oracle's Ubuntu images also ship iptables rules that drop everything
except SSH, and those survive a reboot. Open both, or the cloud console will
show your rules while the host quietly refuses the packets.

```bash
sudo iptables -I INPUT -p tcp --dport 80  -j ACCEPT
sudo iptables -I INPUT -p tcp --dport 443 -j ACCEPT
sudo iptables -I INPUT -p udp --dport 50000:50999 -j ACCEPT
sudo netfilter-persistent save
```

Then the same three, as ingress rules on the subnet's security list, from
0.0.0.0/0.

**Its instances are behind NAT.** The VM sees a private address on its NIC and
never learns the public one, so the SFU offers candidates nobody can reach.
Set it explicitly:

```bash
curl -s ifconfig.me          # the address to put in WEBRTC_PUBLIC_IP
```

Leaving it empty here is the failure the boot log warns about, and the symptom
is that everything works except that nobody can hear anybody.

**A hostname without buying a domain.** Browsers refuse microphone access
outside a secure context, so an IP address alone cannot carry a call however
open the ports are. `nip.io` resolves `1-2-3-4.nip.io` to `1.2.3.4`, and it is
a real name, so Caddy can get a real certificate for it:

```
DOMAIN=203-0-113-45.nip.io
CORS_ORIGINS=https://203-0-113-45.nip.io,tauri://localhost,http://tauri.localhost
```

Swap in your own domain later by editing those two lines and restarting; the
certificate is fetched again and nothing else changes.

---

## Letting CI build it

`make deploy-up` builds the image on the host. On two cores that means
compiling Go and bundling the client every time, so the workflow in
`.github/workflows/publish.yml` does it instead: every push to `main`
builds `linux/arm64` on GitHub's ARM runners and pushes to
`ghcr.io/esuedu/vocalis:latest`.

Nothing is stored in GitHub for this and nothing reaches in. The workflow signs
in with the token Actions already has, and the host only pulls a public image.

Once, after the first successful run, open the package under the repository's
Packages tab and set its visibility to public. Until you do, the host's pull
fails with `denied`, which reads like an authentication problem rather than a
one-time setting.

Then on the host:

```bash
echo 'VOCALIS_IMAGE=ghcr.io/esuedu/vocalis:latest' >> .env
make deploy-pull
```

`deploy-pull` fetches that image and restarts without building. Leave
`VOCALIS_IMAGE` out of `.env` and `make deploy-up` behaves exactly as before,
building locally -- useful when you are testing a change that is not pushed.

The image is arm64 only, because that is the host it is for. Building for amd64
as well means a second runner and a manifest merge, and is worth adding the day
there is a second host rather than before.

---

## Releasing the desktop app

Tagging `v*` builds the installers and attaches them to a draft release:
a universal `.dmg` for macOS, an `.msi` and an `.exe` for Windows.

```bash
git tag v0.1.0
git push origin v0.1.0
```

The build pins the server the installed app talks to, through `VITE_API_URL`.
Without it the app ships pointing at `localhost:8080`, which works only on the
machine that built it. It defaults to this deployment; to point a release
somewhere else, set a repository variable called `VITE_API_URL` rather than
editing the workflow. A pinned build also hides the server field in the app,
so nobody has to be told what to type.

Neither installer is signed, and both systems say so on first launch. macOS
refuses the app until it is opened once from the right-click menu, or its
quarantine flag is removed with
`xattr -dr com.apple.quarantine /Applications/Vocalis.app`. Windows shows a
SmartScreen box that takes *More info* then *Run anyway*. Removing those needs a
paid certificate from Apple and one from a Windows CA, which is a decision about
money rather than about code.

The release is a draft, so nothing is public until you look at the artifacts
and press publish.

---

## Coming back after a reboot

Every container is declared `restart: unless-stopped`, so Docker brings them
back when the machine boots. That covers a reboot and it does not cover much
else: a `docker compose down` removes the containers, and nothing then recreates
them.

`make deploy-autostart` installs a systemd unit that does:

```bash
make deploy-autostart
```

It enables `docker` as well, because the restart policy is worth nothing if the
daemon is not started at boot, and writes a second unit for the observability
stack which it installs but leaves disabled -- enable it with
`sudo systemctl enable --now vocalis-observe` if you want the dashboards up
without asking.

After that the deployment is an ordinary service:

```bash
sudo systemctl status vocalis
sudo systemctl restart vocalis
```

Worth testing the thing you are relying on, rather than assuming it: reboot,
wait, and check that the site answers.

```bash
sudo reboot
# then, once it is back
curl -sI https://your-domain | head -1
```

---

## Watching it run

`make observe-up` starts Prometheus, Grafana, Loki and two exporters beside the
deployment. cAdvisor reports CPU, memory, disk and network per container, so
the server, Postgres and Caddy are each accounted for separately; node-exporter
covers the host itself; Loki collects every container's logs so the server's
errors are searchable next to the graphs.

Grafana binds to loopback and is not published, because a dashboard with a
default password on a public address is a worse problem than no dashboard.
Reach it over SSH:

```bash
ssh -N -L 3000:127.0.0.1:3000 ubuntu@your-host
```

Then <http://localhost:3000>, user `admin`, password from `GRAFANA_PASSWORD` in
`.env`. Both datasources are already configured, and a dashboard named
*Vocalis* is provisioned: host CPU, memory, disk and uptime along the top, then
CPU and memory broken out per container, disk throughput per container, host
network, and the server's logs at the bottom with the errors filtered out of
them. The *Container* dropdown narrows every panel at once.

The dashboard is a file, not something to edit in the browser -- Grafana
reloads `deploy/observe/grafana/dashboards/vocalis.json` every 30 seconds, and
overwrites whatever you changed in the UI. Edit the file.

If panels read *No data*, ask which ones:

```bash
make observe-check
```

It runs every query the dashboard draws and names the exporter behind each
empty one, which is the difference between a metric that moved and an exporter
that never started. Note that it cannot be run from a Mac usefully: cAdvisor
and node-exporter both need real Linux cgroups, so on a developer machine every
container and host panel reports empty and only the Loki panels answer.

The stack costs roughly 400MB of memory and keeps 15 days or 2GB of metrics,
whichever comes first. On a 1GB instance run the deployment without it and use
`make deploy-logs`; on the 24GB Ampere shape it is not worth thinking about.

If the `host` target reads *down* in Prometheus, node-exporter did not get the
mount propagation it needs -- that is a Linux host detail and the reason this
part cannot be tested on a Mac.

If the per-container panels are empty while cAdvisor looks healthy and its
Prometheus target reads *up*, check the cAdvisor version before anything else.
Docker 29 keeps images in containerd, which has no `image/<driver>/layerdb`
directory; cAdvisor before v0.55 looks one up for every container it finds,
treats the miss as fatal, and ends up registering none of them. Its logs say
`failed to identify the read-write layer ID`. Nothing about the failure is
visible from Prometheus, which sees a target that scrapes fine and simply
returns no container series.

---

## What is running

| Piece | What it does |
|-------|--------------|
| `postgres` | The database. Published on `127.0.0.1:5432` only, so it is reachable from the host and nowhere else. |
| `server` | One container: the Go binary plus the built React client, served from the same origin. Runs with **host networking**, because the SFU needs a thousand UDP ports and publishing those through Docker's bridge means a proxy process per port. |
| `caddy` | TLS, HTTP/2, compression, and the certificate. Optional — it lives behind the `tls` profile. |

The server binds `127.0.0.1:8080` by default, so the plain-HTTP port is not
reachable from outside; Caddy is what the internet talks to. If you terminate
TLS somewhere else, set `HTTP_ADDR=:8080` and put your own proxy in front.

Two named volumes hold everything worth keeping: `vocalis_postgres-data` and
`vocalis_vocalis-files` (avatars, icons, attachments).

### Migrations

The server applies pending migrations at boot, holding a Postgres advisory lock
while it does, and logs each one. There is no separate migrate step to forget
during an upgrade. `MIGRATE_ON_START=false` turns it off if you would rather run
`make migrate` yourself.

### If you use a different proxy

Two settings matter, and both fail in ways that look like something else:

- **Request body size.** A message carries up to 10 files of 25 MB, so the
  server accepts a ~251 MB body. nginx defaults to 1 MB and returns 413 on any
  real attachment: `client_max_body_size 260m`. The bundled Caddyfile already
  sets this.
- **`TRUSTED_PROXIES`.** Only listed peers may set `X-Forwarded-For`. Loopback
  covers a proxy on the same host. Anything trusted here can claim to be any
  client, which defeats the per-IP rate limits — widen it only for proxies you
  run yourself.

WebSockets need no special configuration in Caddy. Most other proxies need the
upgrade headers passed through explicitly.

---

## Upgrading

```bash
git pull
make deploy-up           # rebuilds the image and restarts what changed
```

Migrations run as the new container boots. The gateway drops its websockets on
restart and clients reconnect with exponential backoff, so people see a brief
reconnect rather than a login screen — their refresh tokens survive.

`JWT_SECRET` is the one value you cannot rotate quietly: changing it logs
everybody out and invalidates every signed attachment URL still in flight.

---

## Backups

```bash
make deploy-backup       # backups/db-<timestamp>.sql.gz and files-<timestamp>.tar.gz
```

Both halves matter. The database holds the messages; the files volume holds the
pictures they refer to, and a restore of one without the other leaves broken
attachments. Copy them off the host — a backup on the machine you are backing
up is not a backup. There is no scheduled job here; a cron line calling
`make deploy-backup` is the whole of it.

Restoring:

```bash
gunzip -c backups/db-<ts>.sql.gz | docker compose -f docker-compose.prod.yml exec -T postgres psql -U vocalis vocalis
docker run --rm -v vocalis_vocalis-files:/data -v "$PWD/backups:/in" alpine tar xzf /in/files-<ts>.tar.gz -C /data
```

---

## The desktop app

The desktop client is built separately and is not part of this deployment. It
needs to know which server to talk to, set at build time:

```bash
cd client && VITE_API_URL=https://your-domain npm run tauri build
```

Without `VITE_API_URL` the app expects a server on the same origin, which is
right for the browser and wrong for a packaged app.

The packaged app's origin depends on the platform, and `CORS_ORIGINS` needs
both: **`tauri://localhost`** on macOS and Linux, **`http://tauri.localhost`**
on Windows, whose webview is WebView2. Miss the Windows one and the failure is
worse than a rejection -- the preflight is answered without a matching
allow-origin, so the webview never sends the real request, the client never
receives a response, and it reports that the server did not answer. The server
log shows the shape of it plainly: an `OPTIONS` with no method behind it.

```
{"msg":"http","method":"OPTIONS","path":"/api/v1/auth/register","status":204}
    ... and no POST
```

---

## When something is wrong

**Voice connects, then nobody can hear anybody.** Media is not reaching you.
Check the UDP range is open, and that `WEBRTC_PUBLIC_IP` is set if this host
does not hold its public address directly. If both are right and it still
fails, the callers are behind symmetric NATs that need a TURN relay — Vocalis
does not ship one, deliberately; point `ICE_SERVERS` and `TURN_SECRET` at a
coturn started with `use-auth-secret`.

**One person is silent and everybody else is fine.** The server log is the
first place to look, and it now answers this directly. For that person's user
id you get one of three pictures:

| What the log says | What it means |
|---|---|
| `voice: no path for media` | ICE never connected. Their network cannot reach this host and this host cannot reach them. Only a TURN relay fixes it. |
| `media path established` and nothing else | They connected but are sending nothing — microphone permission, or a muted device. |
| `media path established` then `media arriving at the server` | The server has their audio, so anything still wrong is downstream of here. |

The `ours=` field on `media path established` is worth reading: `host` on a
private address means media never left the LAN, `srflx` means it is traversing
a NAT, and `relay` means TURN is carrying it.

**Behind Cloudflare's orange cloud.** The proxy passes HTTPS and websockets and
does not pass WebRTC's UDP at all, so text works and voice never will. Use a
DNS-only (grey cloud) record, or accept a text-only instance.

**Uploads fail at some size.** That is the proxy's body limit, not the server's;
the server says which file was too large and why.

**`/healthz` returns 503.** The server is up and Postgres is not. `docker
compose -f docker-compose.prod.yml logs postgres`.

**Everybody is logged out after a restart.** `JWT_SECRET` changed. Recover it
or accept the logout; there is no other effect.

---

## What this deployment does not have

Stated plainly, because finding out later is worse:

- **No metrics and no log aggregation.** Observation is `docker compose logs`,
  which is issue #52.
- **No TURN server.** Configure-only, on purpose: relaying media is the most
  expensive thing to self-host.
- **No horizontal scaling.** One node, by design, until the pubsub seam is
  swapped.
- **No email.** So no password reset and no address verification: a ban is by
  user id, and a banned person can register again in seconds.
- **No automatic backups.** The command exists; scheduling it is yours.

## Why not just the tunnel

`make share` puts a running instance behind a Cloudflare quick tunnel in one
command, and for text it is genuinely fine for an evening.

**Voice through it is a coin toss, and this is the part worth understanding.**
The tunnel carries signalling only. Cloudflare does not proxy UDP, so media has
to reach the laptop itself, on the address STUN discovered for it — and whether
that works depends on the two networks involved, not on anything in this repo.
It works when the laptop's router keeps one public port for all destinations
and lets the guest's packets in. It fails, silently and only for that guest,
when either side is behind carrier-grade NAT, a corporate firewall, or a mobile
network. The symptom is the confusing one: they join, they see everybody, and
nothing is ever heard.

There is no configuration that fixes this, because the missing piece is a relay.
Either give the guests a TURN server through `ICE_SERVERS` and `TURN_SECRET`, or
deploy properly — a host with its own address and the UDP range open is the
whole point of everything above.

What it is not is durable: your laptop is the server, the address changes, and
nothing restarts on its own. This runbook is for the case where somebody other
than you expects it to be up tomorrow.
