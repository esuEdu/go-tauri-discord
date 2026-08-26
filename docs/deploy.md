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
right for the browser and wrong for a packaged app. Keep `tauri://localhost` in
`CORS_ORIGINS` or the desktop build is refused at the gateway.

---

## When something is wrong

**Voice connects, then nobody can hear anybody.** Media is not reaching you.
Check the UDP range is open, and that `WEBRTC_PUBLIC_IP` is set if this host
does not hold its public address directly. If both are right and it still
fails, the callers are behind symmetric NATs that need a TURN relay — Vocalis
does not ship one, deliberately; point `ICE_SERVERS` and `TURN_SECRET` at a
coturn started with `use-auth-secret`.

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
command, and it is genuinely fine for an evening. Only the signalling goes
through the tunnel; media does not, and reaches the SFU on the candidates STUN
discovered for it, which is why voice has worked from behind a home router
without any of the configuration above.

What it is not is durable: your laptop is the server, the address changes, and
nothing restarts on its own. This runbook is for the case where somebody other
than you expects it to be up tomorrow.
