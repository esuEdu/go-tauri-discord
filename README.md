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
- ⌨️ **Global Push-to-Talk:** Native system keybindings and global hotkeys accessible even while playing full-screen games.
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
- **Voice/Video Routing:** Pion WebRTC / WebSocket signalling *(not yet implemented)*
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
- [x] WebSocket gateway: identify, heartbeat, resume, per-topic fanout
- [x] Text messages with keyset-paginated history
- [ ] Attachments and avatars on S3-compatible storage
- [ ] P2P / SFU WebRTC voice channels
- [ ] High-FPS screen sharing with window selection
- [ ] Push-to-Talk with native global shortcuts
- [ ] Noise suppression & acoustic echo cancellation
- [ ] End-to-end encrypted direct messaging

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.
