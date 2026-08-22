# Vocalis — Design Reference

Everything the app currently is, written for someone designing it rather than
building it. No code, no jargon that isn't explained.

If something here disagrees with the running app, the app is right and this file
is stale — please say so.

---

## 1. What Vocalis is

A chat app for small groups of friends. Text channels, voice channels, and
screen sharing. It runs in a browser and as a desktop app for Mac and Windows.

It is close in shape to Discord, on purpose. People already know how that works,
and copying the shape means the design effort goes into making it feel good
rather than into teaching a new idea.

The whole thing is one screen. There is no navigation, no routing, no back
button. Everything happens in place.

---

## 2. Two words that mean the same thing

The app says **server**. The code says **guild**. They are the same object: a
space with channels in it that people join by invite.

Say **server** in anything a person reads. You will see **guild** in file names
and in conversation with the developer.

---

## 3. The layout

One screen, four regions, fixed widths on the left:

```
┌──────┬──────────────┬─────────────────────────────────────────────┐
│      │              │                                             │
│  68  │     220      │                  fills                      │
│  px  │      px      │                                             │
│      │              │                                             │
│servers│  channels   │        chat  or  voice                      │
│ rail │              │                                             │
│      │              │                                             │
│      │              │                                             │
├──────┴──────────────┴─────────────────────────────────────────────┤
│ status bar — 32px tall, full width                                │
└───────────────────────────────────────────────────────────────────┘
```

- **Servers rail** — 68px. A vertical stack of round buttons, one per server,
  plus a "+" at the bottom.
- **Channel list** — 220px. Server name at the top, then channels, then an
  invite box pinned to the bottom.
- **Main** — everything left over. Shows either the chat view or the voice view,
  depending on which channel is selected.
- **Status bar** — 32px. Connection state on the left, account on the right.

Nothing is responsive yet. There is no mobile layout, no collapsing sidebar, no
breakpoint anywhere. The window is assumed to be desktop-sized.

---

## 4. Colours

These are the only colours in the app. Everything is dark; there is no light
theme and nothing is prepared for one.

| Role | Value | Where it is used |
| --- | --- | --- |
| Background | `#1a1b1e` | The main area, behind chat and voice |
| Raised | `#232428` | Channel list, dialogs, message hover |
| Sunken | `#141517` | Servers rail, status bar, text inputs, empty video |
| Border | `#2e3035` | Hairlines between regions, input outlines |
| Text | `#dcdde1` | Everything readable |
| Muted text | `#8b8d93` | Timestamps, hints, secondary labels |
| Accent | `#5865f2` | Buttons, focus rings, slider fill, active server |
| Danger | `#ed4245` | Delete, disconnect, offline dot, errors |
| Online | `#3ba55d` | Online dot, the "+" new-server button |

**One loose end worth fixing:** the amber used for the "connecting" and
"reconnecting" dot is `#faa61a`, written directly into one rule instead of being
named with the others. If you introduce a warning colour, that is the value it
currently has and the place it needs to come from.

---

## 5. Type

One family: whatever the operating system uses (`system-ui`, falling back to
Segoe UI on Windows and the San Francisco face on Mac). No web font is loaded.
Text is 14px with 1.5 line height unless listed below.

| Size | Used for |
| --- | --- |
| 22px | "Vocalis" on the sign-in card |
| 17px | Dialog title |
| 14px | Everything by default |
| 13px | Error text, dialog body list, form labels |
| 12px | Status bar, screen-tile captions, small labels |
| 11px | Invite link, volume tags, the server address |

Only two weights are in play: normal, and 600 for the server name, message
author and the channel header.

---

## 6. Shape and spacing

**Corner radii:** 4px on channel rows · 6px on buttons, inputs and video tiles ·
10px on the sign-in card and dialogs · 50% on status dots · servers are 16px,
morphing to 12px on hover over 0.12s.

**Gaps and padding** are all multiples of 2 between 2px and 24px. The common
ones are 8px between related things and 12–16px between groups. Buttons are
`8px 14px`, inputs `9px 12px`, the chat composer `12px 16px`, the voice panel
24px, the sign-in card 28px.

There is no spacing scale written down anywhere. If you introduce one, it will
be the first.

---

## 7. Everything on screen today

### Servers rail

Round buttons, 46×46. No icons exist — each shows **the first two letters of the
server name, uppercased**. The active one is filled with the accent colour and
its corners tighten. Below them, a green "+" opens a small inline form to name a
new server.

The data has a slot for a server icon. Nothing uploads or displays one yet.

### Channel list

- Server name at the top, with a hairline under it.
- Channel rows: `#` for text, `🔊` for voice, then the name. The selected one
  gets a darker background and white text.
- **Unread**: a small filled dot on the right of the row. It is a dot only —
  there is no count, and no bold-name treatment.
- At the bottom, pinned: an **Invite a friend** button (which copies a link and
  briefly changes to "Link copied"), the link itself in a small read-only field,
  and a form to join a server by pasting a code.

Channels are a flat list. The data supports **categories** for grouping, and the
app deliberately does not draw them — that is a design decision waiting to be
made, not an oversight.

### Chat view

- Header: `# channel-name`, and the topic after an em dash if there is one.
- Messages: grouped by author. The first of a run shows the author name and the
  time; the rest show only the body, tight underneath. Hovering a message raises
  its background. Your own messages get a small `×` to delete, visible on hover.
  An edited message says "(edited)" after the text.
- Scrolling up far enough loads older messages automatically; there is also a
  **Load older messages** link.
- Composer at the bottom: a single-line input and a **Send** button that is
  disabled while the box is empty. No multi-line, no attachments, no emoji
  picker, no formatting toolbar.

Messages are plain text. Markdown is mentioned in the project description but
nothing renders it.

### Voice view

Replaces the chat view when a voice channel is selected.

- **Screen tiles** at the top when anyone is sharing: 16:9, in a grid that fits
  as many 280px-wide tiles as the space allows. Each has a caption beneath —
  "Your screen" or "*name*'s screen".
- **Member list**: one row per person. A status dot, the name (or "You"), then
  the volume controls.
- **Volume**: a slider per person, 0–200%, with the percentage beside it. When
  that person's screen share is making sound, a **second slider** appears
  underneath, and both get a small tag — `voice` and `screen` — so they can be
  told apart. The screen slider is absent when there is no sound to control.
- **Controls**: Mute/Unmute · Share screen (or "Stop sharing", which becomes
  "Stop sharing (with sound)" when audio is included) · a quality dropdown ·
  Disconnect in red. Before joining, a single **Join voice** button.
- **Notices**, in muted text under the controls, one per situation: you do not
  have permission to share here · this share has no sound and here is how to get
  it · no screen was picked · could not connect, check the microphone
  permission.

### Status bar

Connection dot and word on the left. Then, if the app is pointed at a
non-default server, that address. Then a gap, your username, **Log out**, and
**Delete account** in red.

### Sign-in card

Centred on an otherwise empty screen. "Vocalis" as a heading, a line of context
underneath, then the fields — username only when registering — an error line, a
submit button that becomes "…" while working, and a link to switch between
signing in and registering.

If someone arrives from an invite link, the card switches itself to registering
and the line reads "You have been invited to *server name*."

At the bottom sits the **server address**, either as fixed text or with a
**Change** link that opens a small form. This exists so the app can be pointed
at somebody else's server. **It was styled plainly on purpose and is expected to
be redesigned** — treat it as a placeholder, not a decision.

### Delete account dialog

The only modal in the app. Dark backdrop, dismissed by clicking outside or
pressing Escape. Titled "Delete your account?", followed by three plain
statements of what will happen, a password confirmation, and two buttons: **Keep
my account** and **Delete for good** in red. The destructive one stays disabled
until a password is typed.

---

## 8. Every state that needs a design

The boring list, which is where most of the work usually turns out to be.

**Connection** — connecting · connected · reconnecting · closed. Shown as the
status-bar dot: grey, green, amber, red.

**Voice** — idle · connecting · connected · failed.

**A channel** — normal · selected · unread. (Selected and unread never show
together; opening a channel clears it.)

**A person in voice** — online · offline · you · sharing with sound.

**Messages** — loading · empty ("No messages yet. Say something.") · more to
load · send failed.

**Sharing** — not sharing · sharing with sound · sharing silently · not allowed
to share · picker was dismissed.

**Buttons** — normal · hover · disabled (50% opacity) · focused (2px accent
outline, drawn inside the edge).

**The app as a whole** — booting ("Loading…") · signed out · signed in with no
servers · signed in with no channel selected.

---

## 9. Rules that will bite a design

Things that are true about how the app behaves, which a design has to live with.

1. **Nobody has a picture.** No avatars, no server icons. Servers show two
   letters; people show only a name. The data has slots for both and nothing
   fills them. Any design leaning on avatars is designing a feature that does
   not exist yet.

2. **Going offline takes about 90 seconds.** When someone closes the app they
   stay green for a minute and a half, on purpose — it means a brief wifi drop
   doesn't flicker everyone offline and back. So presence is *slightly* stale by
   design, and a hard "offline" treatment will look wrong sometimes.

3. **Names arrive slightly after ids.** In a few places the app briefly shows
   the first eight characters of an internal id before it learns the username.
   It looks like `a3f9c201`. Worth having a deliberate placeholder rather than
   letting that show.

4. **A screen share can be silent, and that is normal.** On a Mac, only a
   browser *tab* can be shared with its sound; a window or a whole screen never
   carries audio. The app now says so, but the wording is functional and could
   be much better.

5. **Screen sharing has four quality presets** — Light, Smooth, Sharp, High —
   currently a plain dropdown reading "Smooth — 720p 60fps". Most people will
   not know what to pick.

6. **Unread has no count.** The server tells the app *whether* there is anything
   new, not how much. A badge with a number would need new work on the server
   first — worth knowing before designing one.

7. **Roles and permissions are fully built and completely invisible.** The
   server understands twelve permissions — view, send, manage messages, manage
   channels, manage roles, kick, ban, connect, speak, share screen,
   administrator, create invites, manage server — and there is no interface for
   any of it. Only the effects show, as things being absent or a notice saying
   you cannot.

8. **Typing indicators are sent and ignored.** The server announces when
   somebody is typing. Nothing draws it. It is free to add.

---

## 10. What does not exist yet

Not designed, not built. Roughly in the order they are likely to matter:

- Any way to see or manage **roles and permissions**
- A **member list** for a server (the app knows who is in it, and only uses that
  in voice)
- **Avatars and server icons**, including uploading them
- **Attachments** — images and files in messages
- **Notifications** of any kind, in-app or system
- A **settings** screen — microphone choice, appearance, anything
- **User profiles** — clicking a name does nothing
- **Categories** for grouping channels
- **Search**, **pins**, **replies**, **reactions**, **threads**
- **Direct messages**
- Any **light theme**, and any **mobile or narrow layout**

---

## 11. Where the design effort probably pays best

An opinion, not a requirement.

The **voice view** is the most functional-looking part of the app and the part
people will spend the most attention on when it matters. It currently reads as a
list of controls stacked in a box.

The **channel list and servers rail** are what people look at constantly, and
two-letter squares are the weakest thing on screen.

The **sign-in card and the server address** are the first thing anyone new
sees, and the server address in particular is a raw placeholder.

Everything else works and can wait.
