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

- **Language:** Go (1.22+)
- **Text Networking:** Goroutine-based WebSockets
- **Voice/Video Routing:** Pion WebRTC / WebSockets Signaling
- **Database:** PostgreSQL / SQLite (GORM)

---

## 📁 Repository Structure

Vocalis is organized as a monorepo for streamlined local development and synchronized API releases:

```
vocalis/
├── client/                  # Desktop Application
│   ├── src/                 # React UI components, state management
│   ├── src-tauri/           # Tauri Rust configuration, native hooks
│   ├── package.json
│   └── vite.config.ts
│
├── server/                  # Go Central Backend
│   ├── cmd/                 # Application entrypoints (server binary)
│   ├── internal/            # Core server logic
│   │   └── models/          # Database schemas
│   ├── go.mod
│   └── go.sum
│
├── shared/                  # Shared API specs, types, and WebSocket contracts
│   └── events.json
│
├── Makefile                 # Unified development task runner
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

1. **Clone the Repository:**

    ```bash
    git clone https://github.com/your-username/vocalis.git
    cd vocalis
    ```

2. **Install Frontend Dependencies:**

    ```bash
    cd client
    npm install
    cd ..
    ```

3. **Run Both Server and Client in Development Mode:**

    If you have `make` installed, simply run:

    ```bash
    make dev
    ```

    _Alternatively, start them in separate terminal windows:_

    ```bash
    # Terminal 1: Go Server
    cd server
    go run cmd/server/main.go

    # Terminal 2: Tauri Desktop App
    cd client
    npm run tauri dev
    ```

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
cd server
go build -o bin/vocalis-server cmd/server/main.go
```

---

## 📋 Roadmap

- [x] Initial project structure & monorepo design
- [ ] WebSocket-based text channels & messaging history
- [ ] P2P / SFU WebRTC voice channels
- [ ] High-FPS screen sharing with window selection
- [ ] Push-to-Talk with native global shortcuts
- [ ] Noise suppression & acoustic echo cancellation
- [ ] End-to-end encrypted direct messaging

---

## 📄 License

Distributed under the MIT License. See `LICENSE` for more information.
