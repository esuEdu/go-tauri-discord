# go-tauri-discord

> A modern, ultra-lightweight, and high-performance communication platform featuring real-time text chat, crystal-clear voice channels, and high-framerate screen sharing. Built natively for desktop.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Tauri Version](https://img.shields.io/badge/Tauri-v2-24C8D8?logo=tauri&logoColor=white)](https://tauri.app/)
[![React Version](https://img.shields.io/badge/React-18+-61DAFB?logo=react&logoColor=black)](https://react.dev/)

---

## 🌟 Overview

**go-tauri-discord** is an open-source, resource-efficient alternative to traditional chat platforms. By leveraging **Tauri (Rust)** for native desktop webviews and **Go** for concurrent real-time networking, Vocalis delivers a full-featured voice, text, and screen sharing experience while using up to **80% less memory** than Electron-based applications.

---

## ✨ Key Features

- 💬 **Real-time Text Channels:** Low-latency chat powered by Go WebSocket goroutine hubs, supporting markdown formatting and channel rooms.
- 🎙️ **Voice Channels:** Crystal-clear multi-party audio powered by WebRTC and low-overhead audio encoding.
- 🖥️ **Screen Sharing:** Ultra-low latency system and window capture via WebRTC media streams with customizable framerates and resolutions.
- ⚡ **Minimal Memory Footprint:** Idles at ~30MB–50MB RAM compared to 300MB+ in typical web-wrapped desktop apps.
- ⌨️ **Global Push-to-Talk:** _Planned._ Native system keybindings and global hotkeys accessible even while playing full-screen games.
- 🛡️ **Self-Hostable Architecture:** Run your own server node using a single lightweight Go executable.

---

## 🏗️ Tech Stack

```
+-------------------------------------------------------------------+
|                        Vocalis Client App                         |
|  +-------------------------------------------------------------+  |
|  | Tauri v2 (Rust Shell + Webview UI: React/TypeScript)        |  |
|  | - Low-overhead native UI rendering                          |  |
|  | - System tray, screen picker, global hotkeys                |  |
|  +------------------------------+------------------------------+  |
+---------------------------------|---------------------------------+
                                  | (HTTPS / WSS / WebRTC)
                                  v
+-------------------------------------------------------------------+
|                       Vocalis Server Node                         |
|  +-----------------------+ +-----------------+ +---------------+  |
|  | REST API              | | WebSocket Hub   | | WebRTC        |  |
|  | (Auth, Guilds, DB)    | | (Text Chat)     | | Signaling/SFU |  |
|  +-----------------------+ +-----------------+ +---------------+  |
+-------------------------------------------------------------------+
```

### **Client (Desktop)**

- **Framework:** [Tauri v2](https://tauri.app/) (Rust)
- **UI / Frontend:** React, TypeScript, Tailwind CSS
- **Media Capture:** WebRTC APIs, OS Desktop Duplication

### **Server (Backend)**

- **Language:** Go (1.26+)
- **Architecture:** Modular monolith — one binary, feature packages, interfaces only at the seams that will actually be cut later
- **Text Networking:** One multiplexed WebSocket per client (`/gateway`), goroutine-based fanout routed per topic
- **Voice/Video Routing:** Pion WebRTC SFU with WebSocket signalling
- **Database:** PostgreSQL 17 with `pgx` + [`sqlc`](https://sqlc.dev) (compile-time checked SQL, no ORM)
- **Migrations:** [`goose`](https://github.com/pressly/goose), pinned as a Go tool dependency

**Why Postgres and not SQLite:** a chat server issues many small concurrent
writes (messages, presence, read state, voice state) and SQLite serialises
every writer. Postgres additionally provides `LISTEN/NOTIFY`, `JSONB`,
full-text search, and partitioning for when `messages` grows. Supporting both
dialects would double the migration and query surface for no user-visible
gain.

**Why no ORM:** the hot path is keyset pagination over messages joined to
authors and attachments — exactly where an ORM produces N+1 queries. `sqlc`
generates type-safe Go from the same SQL that ships, so schema drift is a
build failure rather than a runtime one.

---

## 📁 Repository Structure

Vocalis is organized as a monorepo for streamlined local development and synchronized API releases:

```
go-tauri-discord/
├── client/                       # Desktop application
│   └── src/types/events.gen.ts   # Generated from server/pkg/events — do not edit
│
├── server/
│   ├── cmd/api/                  # The one binary: REST API + gateway
│   ├── internal/
│   │   ├── config/               # Env -> typed config, loaded once
│   │   ├── domain/               # Entities, error kinds, permission bitfield
│   │   ├── auth/                 # Register/login, JWT + refresh rotation
│   │   ├── guild/                # Guilds, channels, members, roles, permissions
│   │   ├── message/              # History, keyset pagination
│   │   ├── gateway/              # WebSocket hub: sessions, replay, fanout
│   │   ├── db/
│   │   │   ├── migrations/       # goose SQL migrations
│   │   │   ├── queries/          # sqlc source SQL
│   │   │   └── gen/              # sqlc output — do not edit
│   │   └── platform/
│   │       ├── httpx/            # Router, middleware, error mapping
│   │       ├── bus/              # Event publish side
│   │       └── pubsub/           # Broker interface + in-memory impl
│   ├── pkg/events/               # Wire contract — source of truth for both sides
│   ├── sqlc.yaml
│   └── tygo.yaml
│
├── docker-compose.yml            # Postgres (+ MinIO behind a profile)
├── Makefile                      # Unified development task runner
└── README.md
```

---

## 🚀 Getting Started

### Prerequisites

Ensure you have the following tools installed on your development machine:

1. **Go** (v1.22 or higher) – [Download](https://go.dev/dl/)
2. **Node.js** (v18+ / pnpm or npm) – [Download](https://nodejs.org/)
3. **Rust Toolchain** (required by Tauri) – [Install Rust](https://www.rust-lang.org/tools/install)
4. System dependencies for Tauri (Linux only: `libwebkit2gtk-4.1-dev`, `build-essential`, `curl`, `wget`, `file`, `libssl-dev`, `libgtk-3-dev`, `libayatana-appindicator3-dev`, `librsvg2-dev`).

---

### Quick Start (Development)

1. **Clone and configure:**

    ```bash
    git clone https://github.com/esuEdu/go-tauri-discord.git
    cd go-tauri-discord
    cp .env.example .env
    ```

2. **Start Postgres, run migrations, and boot the server:**

    ```bash
    make dev
    ```

    This starts the `postgres` container, waits for it to become healthy,
    applies migrations, and runs the API on `:8080`. Check it with
    `curl localhost:8080/healthz`.

3. **Run the desktop client** (separate terminal):

    ```bash
    cd client && npm install
    make dev-client
    ```

### Common tasks

Run `make help` for the full list.

| Command | What it does |
| --- | --- |
| `make dev` | Postgres + migrations + server |
| `make migration name=add_reactions` | Scaffold a new migration |
| `make migrate` / `make migrate-down` | Apply / roll back migrations |
| `make db-reset` | Drop the volume and rebuild the schema |
| `make sqlc` | Regenerate the query layer from SQL |
| `make types` | Regenerate client TypeScript from `pkg/events` |
| `make check` | Vet, format check, and `go test -race` |
| `make e2e` | Drive the real API and gateway against Postgres, no client needed |

`goose`, `tygo` and `sqlc` need no global install — the first two are pinned
as Go tool dependencies in `server/go.mod`.

---

## 🔌 Architecture Notes

### Authentication

Two tokens. The **access token** is a short-lived HS256 JWT verified locally
on every request, so the hot path never touches the database. The **refresh
token** is opaque random bytes stored only as a SHA-256 hash and rotated on
every use; presenting a revoked token is treated as theft. The desktop client
keeps the refresh token in the OS keychain, never in `localStorage`.

### Account deletion

`DELETE /api/v1/users/@me` asks for the password again. An access token is
enough to read an account and should not be enough to destroy one, and the
request is irreversible.

**Messages survive; authorship does not.** They are reassigned to a reserved
`Deleted User` row seeded by migration, which is why `messages.author_id` can
keep its `ON DELETE RESTRICT`: nothing is deleted, so nothing has to be
weakened. Erasing the content instead would take half of every conversation
away from the people still in the channel, who did not ask to be forgotten.
That row is real but unusable — its password hash is empty, so no password can
ever match it, and it holds the name `Deleted User` against the unique index so
nobody can register it and impersonate the absence.

A guild owned by the account is handed to its highest-ranked remaining member,
earliest joined breaking the tie. If nobody is left it is deleted rather than
left ownerless and unreachable. Everything else that points at the account —
refresh tokens, memberships, roles, read states, invites — is `ON DELETE
CASCADE`, so the row disappearing is the erasure. All of it is one transaction:
a half-deleted account is worse than either outcome.

Live gateway sessions are dropped explicitly. A session authenticates once at
identify and is never checked again, so without that the deleted account would
keep receiving messages until its socket happened to close.

### The gateway

One WebSocket per client at `GET /gateway`, multiplexing every event that
client may see — not one connection per channel. Frames share an envelope:

```jsonc
{ "op": 0, "t": "MESSAGE_CREATE", "s": 42, "d": { /* payload */ } }
```

| Op | Direction | Meaning |
| --- | --- | --- |
| 0 | server → client | Dispatch (carries `t` and `s`) |
| 1 | client → server | Heartbeat |
| 2 | client → server | Identify (authenticate a new session) |
| 3 | server → client | Hello (sent immediately on connect) |
| 4 | server → client | Heartbeat ack |
| 5 | client → server | Resume (replay missed events) |
| 6 | server → client | Invalid session — re-identify from scratch |

Handshake: the server sends **HELLO**, the client replies **IDENTIFY** (or
**RESUME**), and the server answers with **READY** — always the first
dispatch — followed by live events.

`s` is a per-session sequence number. Sessions outlive their connection by 90
seconds, keeping a 256-frame replay buffer, so a brief network drop is
recovered with **RESUME** rather than a full refetch. Clients must ignore any
frame whose `s` is not greater than the last one processed.

Fanout is routed **per topic, not per session**: one broker subscription and
one forwarding goroutine exist per active topic regardless of how many
sessions listen. A client that falls more than 256 frames behind is
disconnected on purpose — it reconnects and resumes, which is far cheaper
than letting one slow client stall delivery for everyone else.

### Scaling past one node

`internal/platform/pubsub.Broker` is the seam. Today it is an in-process
implementation; swapping in NATS (whose server embeds into this binary,
preserving the single-executable story) is the only change needed — nothing
in the feature packages moves.

### Permissions

A 64-bit bitfield on roles, plus per-channel allow/deny overwrites, resolved
in `domain.ResolvePermissions` in this order: guild owner and Administrator
short-circuit to everything; role permissions are unioned; role overwrites
apply denies before allows; the member-specific overwrite wins last.

Every send, edit, delete, typing indicator and history read is authorised
first, so the resolution is **one query**: `ResolveChannelAccess` fetches the
channel, the guild's owner, the membership and both the role and overwrite sets
together, aggregating the two variable-length sets as JSON so they survive in a
single row. Caching them instead would have been faster still and wrong more
often — a revoked role has to be denied on the next request, not at the end of
a TTL, and invalidating a cache across nodes is the harder half of a problem
this design does not have.

#### Writing them

| Method | Path | Needs |
| --- | --- | --- |
| `GET` | `/api/v1/guilds/{guild}/roles` | membership |
| `POST` | `/api/v1/guilds/{guild}/roles` | `ManageRoles` |
| `PATCH` `DELETE` | `/api/v1/roles/{role}` | `ManageRoles` |
| `GET` | `/api/v1/guilds/{guild}/members/{user}/roles` | membership |
| `PUT` `DELETE` | `/api/v1/guilds/{guild}/members/{user}/roles/{role}` | `ManageRoles` |
| `GET` | `/api/v1/channels/{channel}/overwrites` | membership |
| `PUT` `DELETE` | `/api/v1/channels/{channel}/overwrites/{target}` | `ManageRoles` |

Two rules hold every one of these together, and `ManageRoles` is worthless
without both:

**Position.** You may only touch a role *strictly below* your own highest one.
Equal is not below, so a moderator cannot edit, move, delete or hand out the
role that makes them a moderator. `@everyone` sits at position 0 and every
other role starts at 1.

**Subset.** You may only grant, revoke or overwrite a permission you hold
yourself. Without this the position rule alone would be theatre: create a role
at position 1, give it Administrator, assign it to yourself, and the hierarchy
you were just under no longer exists. Editing a role compares the bits that
*changed*, so someone may reword a role they could not have created.

The guild owner is exempt from both. `@everyone` cannot be renamed, moved,
deleted or assigned — only its permissions can change, which is how the guild's
baseline is set. Channel overwrites additionally refuse to carry Administrator,
refuse to allow and deny the same bit, and refuse bits that do not exist.

#### When a change takes effect

Nothing is cached, so **HTTP is immediate**: the request after a revocation is
already refused, which `TestRevokingViewChannelTakesEffectOnTheNextRequest`
pins down.

The gateway is a different story, and worse than "eventually":

- **`READY` is correct.** A session that identifies after losing `ViewChannel`
  is not told the channel exists.
- **Fanout is not.** Sessions subscribe to a topic *per guild*, and
  `MESSAGE_CREATE`, `MESSAGE_UPDATE`, `MESSAGE_DELETE` and `TYPING_START` are
  published there with no per-channel filter. So a member who cannot view a
  channel still receives its messages on the socket — not until they reconnect,
  but indefinitely, including on a connection opened long after the channel was
  made private.

A `ViewChannel` deny is therefore an HTTP-level restriction, not
confidentiality. `TestGatewayFanoutIgnoresChannelVisibility` asserts the leak
so that closing it fails the suite instead of passing silently. Closing it
means filtering fanout per session per channel, which needs a permission set
the gateway does not currently keep.

Voice is fixed at join for a different reason: the SFU reserves a member's
video transceiver when they connect, so revoking `Stream` mid-call does not
retract the one they already have. It applies on rejoin.

### Desktop client

```bash
make client-install     # once
make dev                # terminal 1: Postgres, migrations, API on :8080
make dev-client         # terminal 2: native window (needs Rust)
```

No Rust yet? `make dev-web` serves the same UI at http://localhost:1420 in a
browser. The Tauri shell wraps that identical Vite app, so nothing is wasted
by starting there. Install Rust with:

```bash
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

`make client-build` produces `Vocalis.app` and a `.dmg` under
`client/src-tauri/target/release/bundle/` — around 3 MB.

The packaged app serves its frontend from `tauri://localhost`, so it cannot
use the page origin to find the API the way the browser build does. It falls
back to `http://localhost:8080`. Point it elsewhere with `VITE_API_URL` at
build time, or by setting `server_url` in local storage at runtime.

The client covers registration and login, creating and joining servers,
channel switching, message history with scroll-back paging, sending and
deleting, and live updates over the gateway with a connection indicator in the
status bar.

`src/gateway.ts` owns the socket and hides reconnection from the UI: it tracks
the sequence number, RESUMEs after a drop, discards replayed duplicates, and
backs off exponentially with jitter so a server restart does not produce a
retry storm.

**Adding a friend to a server** uses invite links. *Invite a friend* copies a
link carrying an 8-character code; opening it previews the server and joins
automatically after registration. Invites can be limited by use count and
expiry, and revoked without removing anyone who already joined.

### Voice

Voice channels carry audio through a Pion SFU. Each participant holds one peer
connection to the server, which forwards their audio to everyone else, so a
channel of N members costs N connections rather than N squared.

The server is always the offerer, which removes SDP glare entirely: clients
only ever answer. Joining requires the `Connect` permission on the channel.
`ICE_SERVERS` configures STUN, and `VOICE_DISABLED=true` turns voice off.

### Screen sharing

A screen rides as a second track on the same peer connection, so sharing costs
no extra ICE negotiation and stops when the call does.

Because the server is the only offerer, a client cannot renegotiate on its own.
Two things make that workable:

- On join the SFU reserves a **recvonly video transceiver** for the member, and
  puts its `mid` on every offer. The client binds its capture to that exact
  transceiver rather than guessing at m-line order, which matters once other
  people's screens are arriving on video sections of their own.
- Starting or stopping a share sends `VOICE_SCREEN`, and the server answers
  with a fresh offer that adds or withdraws the forwarded track.

The sharer has to say it in words because the transport never does. A browser
that stops sharing calls `replaceTrack(null)` and simply stops sending on the
same SSRC: the track does not end, so the server's read blocks forever and
would go on believing the share is live. Viewers would keep the last frame on
screen for the rest of the call. Nothing in the media path distinguishes a
stopped share from a screen that has not changed, which is why saying so is a
message rather than an inference.

Withdrawing does not throw the forwarded track away, because the browser
resumes on the same SSRC when the member shares again — the SFU would never see
a new track to forward. The track is taken out of the room and put back, and
viewers see it appear and disappear.

The transceiver is only reserved for members holding the `Stream` permission.
Without it there is no video section in the offer at all, so the SDP itself
refuses the share rather than a check that could be forgotten.

`VOICE_SCREEN_UPDATE` announces who is sharing, keyed by stream id, because
nothing in a forwarded track says whose it is — the viewer needs it to put a
name on a tile. Joiners are told about shares already in progress.

Capture uses `getDisplayMedia`, so the window and display picker is the
browser's. Webviews that do not implement it cannot share; the browser build
can.

Capture runs to a budget the sharer picks, because the right trade depends on
what is on the screen and on the link carrying it. Each preset fixes a
resolution, a framerate, a bitrate ceiling, a `contentHint` and a
`degradationPreference` together, since setting any one of them without the
others just moves where the encoder cheats:

| Preset | Capture | Ceiling | Under pressure |
| --- | --- | --- | --- |
| Light | 720p 30fps | 1.2 Mbps | keeps resolution |
| **Smooth** (default) | 720p 60fps | 3 Mbps | keeps framerate |
| Sharp | 1080p 30fps | 4 Mbps | keeps resolution |
| High | 1080p 60fps | 8 Mbps | keeps resolution |

A ceiling is not a floor. WebRTC's own bandwidth estimate decides what actually
goes out, so raising these lets a good link spend more without obliging a bad
one to try; Light stays low on purpose, for the link that cannot.

The default is smooth rather than sharp because a share that stutters reads as
broken, while one that is slightly soft only reads as a screen share. Changing
the preset mid-share needs no renegotiation: `applyConstraints` retunes the
capture and `setParameters` the encoder, both on a track already flowing. The
choice is remembered in `localStorage`.

Keyframes are only sent when somebody needs one — when a viewer's answer is
applied, and when a viewer's own decoder asks, which the SFU relays to the
publisher no more than twice a second. A screen that nobody has just subscribed
to costs nothing, which is where a static share's bitrate goes into detail.

### Sharing a running instance

The Go server can serve the built UI, so the whole app lives on one port and
one tunnel exposes it:

```bash
make share
```

That builds the UI, serves it with the API on one port, opens a Cloudflare
tunnel, and prints the public link:

```
  Send this to your friends:
      https://classroom-asthma-carried-ascii.trycloudflare.com
```

Send that URL to anyone. No CORS configuration and no rebuild are needed: the
client talks to whatever origin it was served from, and the gateway authorises
a websocket whose `Origin` matches the request `Host`. Ctrl-C stops the server
and the tunnel together. `make serve` does the same without exposing anything.

The URL is random and changes on every restart, so send the current one. A
stable hostname needs a named tunnel and a free Cloudflare account.

Your friends open the link, register, and they are in. Click **Invite a
friend** in the sidebar to copy a link like
`https://<host>/?invite=LBJJqars` — opening it shows which server they have
been invited to, and joining happens automatically once they have an account.
An invite code can also be pasted directly into the **invite code** box.

Requires `cloudflared` (`brew install cloudflared`).

`make share` creates `.env` with a generated `JWT_SECRET` on first run. That
matters — the development fallback secret is committed to this public
repository, so an instance exposed with the default would let anyone forge a
token for any account.

Registration, login, messages and typing are rate limited, so a shared link is
no longer wide open. It is still an instance on your laptop behind a tunnel
that terminates TLS at Cloudflare — fine among friends, not fine in public.

### Testing without the desktop client

`make e2e` starts the real server in-process against Postgres and drives it
over HTTP and a genuine websocket — registration, guild creation, keyset
pagination, permission boundaries, gateway IDENTIFY/READY, live fanout, and
RESUME replay. It brings the database up and migrates first, so a single
command is enough:

```bash
make e2e
```

The suite is behind the `e2e` build tag, so `make test` stays fast and
database-free. Usernames are randomised per run, so repeated runs against the
same database do not collide.

### Rate limiting

Token buckets, keyed by account for authenticated routes and by client IP for
public ones. Exceeding a limit returns `429` with `Retry-After`.

Keying public routes on IP only works if the server knows the real client
address. Behind `make share`, cloudflared connects from loopback, so without
trusted-proxy handling every visitor would share one bucket and the first few
registrations would lock out everyone else. `TRUSTED_PROXIES` defaults to
loopback and the client address is read from `CF-Connecting-IP` or the
rightmost untrusted hop of `X-Forwarded-For`. Peers outside that list have
their headers ignored, so the limit cannot be bypassed by claiming a different
address.

Failed attempts consume tokens. Wrong passwords and rejected registrations are
exactly what an attacker generates, so making them free would defeat the
limit. Login is additionally throttled per account, because throttling by IP
alone does not stop guessing distributed across many addresses.

Limits are set with `REGISTER_PER_HOUR`, `LOGIN_PER_MINUTE`,
`MESSAGES_PER_MINUTE` and `MAX_SESSIONS_PER_USER`; `RATE_LIMIT_DISABLED=true`
turns the whole thing off for local work.

Buckets live in memory, which shares the single-node constraint of the event
broker — both move to a shared backend together when a second node appears.

### Preventing client/server drift

Wire types are defined once in `server/pkg/events` and generated into
`client/src/types/events.gen.ts` by `make types`. `make verify-generated`
fails CI when the checked-in output is stale.

---

## 📦 Building for Production

### Build Desktop Client (`.msi`, `.exe`, `.dmg`, `.AppImage`)

```bash
cd client
npm run tauri build
```

Built installers and binaries will be output to `client/src-tauri/target/release/bundle/`.

### Build Backend Server Binary

```bash
make build   # -> server/bin/vocalis-server
```

`JWT_SECRET` (32+ bytes) is required when `ENV=production`; the server
refuses to start without it. Generate one with `openssl rand -base64 48`.

---

## 📋 Roadmap

- [x] Initial project structure & monorepo design
- [x] Postgres schema, migrations, and type-safe query layer
- [x] Auth: registration, login, JWT access + rotating refresh tokens
- [x] Guilds, channels, members, roles, permission resolution
- [x] Role and overwrite management, with a hierarchy that cannot be climbed
- [ ] Gateway fanout filtered per channel, so a private channel really is
- [x] WebSocket gateway: identify, heartbeat, resume, per-topic fanout
- [x] Text messages with keyset-paginated history
- [x] Account deletion: authorship reassigned, guilds inherited, sessions cut
- [ ] Attachments and avatars on S3-compatible storage
- [x] SFU WebRTC voice channels
- [x] Screen sharing with window selection and picked quality presets
- [x] Screen capture in the macOS desktop app, by enabling it in WKWebView
- [ ] Screen capture proven on the Windows and Linux desktop builds
- [ ] Per-viewer quality adaptation, so one weak link is only their problem
- [ ] Push-to-Talk with native global shortcuts
- [x] Echo cancellation and noise suppression as the browser provides them
- [ ] Dedicated noise suppression that beats what the browser does
- [ ] End-to-end encrypted direct messaging

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.
