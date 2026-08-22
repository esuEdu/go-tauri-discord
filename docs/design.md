# Vocalis — What needs designing

A list of everything the app has to say and every situation it can be in, so it
can be designed freshly. This is not a description of how it looks today, and
nothing here is a layout — the arrangement is yours.

Vocalis is a chat app for small groups of friends: text channels, voice
channels, and screen sharing. It runs in a browser and as a desktop app.

Two words for one thing: people see **server**, the code says **guild**. They
are the same — a space with channels in it, joined by invite.

---

## 1. Sign in

**Fields**

| Field | Rules |
| --- | --- |
| Email | Must be a real email address |
| Password | 8–72 characters |

**What can go wrong, and what we say now**

| Situation | Current wording |
| --- | --- |
| Wrong email or wrong password | "invalid credentials" |
| Too many attempts on one account | "too many sign-in attempts for this account" |
| Already signed in on too many devices | "too many concurrent sessions for this account" |
| The server cannot be reached | "could not reach the server at *address*" |

Two things worth knowing:

- **Wrong email and wrong password give the same message on purpose.** Saying
  "no such account" would let a stranger discover who has one. The wording can
  change; the vagueness cannot.
- **Sign-in is limited to 20 attempts per minute** per account, and a person is
  allowed **5 devices signed in at once**. Both need a way to be explained when
  they happen — the second one especially, because it is confusing.

**States**

Empty · being filled in · submitting (nothing can be typed, and something has to
show it is working) · rejected with a message · succeeded.

---

## 2. Register

**Fields**

| Field | Rules |
| --- | --- |
| Username | 2–32 characters. Any characters, including spaces and emoji |
| Email | Must be a real email address |
| Password | 8–72 characters. No complexity requirement — length is the only rule |

**What can go wrong**

| Situation | Current wording |
| --- | --- |
| Username too short or too long | "username must be 2-32 characters" |
| Email not an email | "invalid email address" |
| Password too short or too long | "password must be 8-72 characters" |
| Username or email already used | "username or email already taken" |
| Too many accounts made from one place | rate limited, 10 per hour |

**Worth deciding:**

- The "already taken" message deliberately does not say **which** one — same
  reason as above. If you want it to say which, that is a decision to make
  knowingly.
- Nothing is checked until the form is submitted. Whether rules are shown
  up front, as you type, or only on failure is open.
- There is no email confirmation, no password strength meter, no "show
  password", and no forgotten-password flow at all. **There is currently no way
  for someone to recover a lost password.** Worth designing, or worth deciding
  to leave.

**States**

Same five as sign-in, plus: a person can arrive here from an invite link, which
is its own thing —

---

## 3. Arriving from an invite

Someone is sent a link. Opening it shows the sign-up form with one extra piece
of information: **the name of the server they were invited to**.

They register, and are put into that server automatically. If they already have
an account, they can switch to signing in and are still put into the server.

**States**

- Invite is good — show the server name
- Invite is expired, used up, or was revoked — currently the app says nothing
  and the name simply never appears. **This needs designing.**
- Already a member of that server — nothing special happens today

---

## 4. What we know about a person

This is everything the app holds about someone. Anything not on this list cannot
be shown, because it does not exist.

| | |
| --- | --- |
| **Username** | 2–32 characters, chosen at sign-up. Cannot be changed anywhere in the app |
| **Status** | **Online or offline. That is all.** |
| **Picture** | None. There is a slot for one; nothing uploads or shows it |

**Status is only two values.** Idle, do-not-disturb, invisible, custom status,
"playing something" — none of these exist. If any of them should, that is new
work on the server, not just a design.

Two behaviours that will affect how status looks:

- **Going offline is delayed by about 90 seconds.** When someone closes the app
  they stay online for a minute and a half, deliberately, so a brief wifi drop
  does not flicker them offline and back. Status is therefore a little stale by
  design.
- **A name can arrive slightly after the person does.** In a few moments the app
  knows someone is there before it knows their name, and shows eight characters
  of an internal id — `a3f9c201`. A deliberate placeholder would be better than
  letting that show.

---

## 5. Being in a call

The part you asked about specifically.

### What you see about another person in a call

| | |
| --- | --- |
| Their **name** | Or the id placeholder, briefly |
| **Online or offline** | The only two |
| **Their volume**, for you | 0–200%, yours alone, remembered between calls, never shown to them |
| **Their screen volume**, separately | 0–200%, and only when their share is actually making sound |
| **Whether they are sharing a screen** | And their screen itself, if so |
| **Whether they are talking right now** | Measured from the audio arriving, so it is honest about what you can actually hear |
| **Whether they have muted themselves** | Deliberately silent, as opposed to merely not talking |

Yourself appears in the same list, as **You**, with no volume controls.

### What you cannot see about them, and why it matters

These are gaps, not oversights to design around silently. Every one of them is
something people will expect:

- **Whether they can hear you** (deafened).
- **Their connection quality.**

Neither is as immediately confusing as talking and muting were, and both need
server work before they can be drawn.

### Who is in a voice channel before you join

**You mostly cannot see this.** If someone joins while you are online and
watching, you learn about it. If they were already in the channel before you
opened the app, they are invisible to you until you join yourself.

So a design that shows the members under each voice channel — the way people
expect — needs server work to be reliable. Say so early if you want it.

### Call states

| | |
| --- | --- |
| Not in the channel | Only a way in |
| Connecting | |
| Connected | |
| Failed | Currently: "Could not connect. Check that the microphone permission was granted." |
| Alone in the channel | Currently: "Nobody is here yet." |

Also needed: **your own** microphone on or off, and a way out of the call.

---

## 6. Screen sharing

### What the person sharing needs

| | |
| --- | --- |
| A way to start and stop | |
| **Whether their share has sound** | See below — this matters more than it sounds |
| **Quality**, four choices | Light · Smooth · Sharp · High. Currently a dropdown reading "Smooth — 720p 60fps", which most people cannot choose between |

**The sound problem.** A screen share is often silent, and legitimately so: on a
Mac only a browser **tab** can be shared with its audio — a window or a whole
screen never carries any. People hit this and assume the app is broken. It has
already happened once here.

So a share needs to say clearly whether sound is included, and when it is not,
what would have worked. The current wording is functional and could be much
better.

### What a viewer sees

The screen itself, and **whose it is**. Nothing else — no way to make it
fullscreen, no way to pop it out, no way to hide one share while watching
another, and no way to choose to receive lower quality on a weak connection.

### States

Not sharing · sharing with sound · sharing silently · not allowed to share here
· the person opened the picker and cancelled · someone else is sharing · several
people are sharing at once.

---

## 7. Everything else that needs states

**Connection to the server** — connecting · connected · reconnecting after a
drop · disconnected. This is always visible somewhere, because the app is
useless when it is not connected.

**Starting up** — the moment before the app knows who you are.

**A channel in the list** — normal · the one you are reading · **has something
unread**. Unread is a yes/no; the server does not say how many, so a number
badge would need new work.

**Messages** — loading · none yet · older ones available to load · failed to
send. Messages are plain text, up to 4000 characters, at 60 per minute. No
images, no files, no emoji picker, no replies, no reactions, no editing in the
interface (the server supports editing; nothing offers it).

**Making a server** — a name, 1–100 characters. Nothing else: no picture, no
description, no template.

**Joining a server** — by pasted code, or by opening a link.

**Inviting someone** — produces a link to copy. It can be limited by number of
uses and by expiry, though nothing in the interface offers either yet.

**Deleting an account** — needs the password, and must state plainly that
messages stay behind under "Deleted User", that owned servers pass to another
member, and that every device is signed out. It cannot be undone.

---

## 8. All the limits in one place

| Thing | Limit |
| --- | --- |
| Username | 2–32 characters |
| Password | 8–72 characters |
| Server name | 1–100 characters |
| Channel name | Required |
| Message | 4000 characters |
| Messages sent | 60 per minute |
| Sign-in attempts | 20 per minute per account |
| Accounts created | 10 per hour |
| Devices signed in at once | 5 |
| Volume range | 0–200% |
| Going offline | ~90 seconds after closing the app |

---

## 9. Things that exist but nobody can see

Built and working on the server, with no interface at all. Any of these is
available to design without new server work:

- **Roles and permissions.** Twelve of them — who can post, delete other
  people's messages, make channels, manage roles, kick, ban, join voice, speak,
  share a screen, invite, and administer. All enforced. None visible. Today the
  only sign is a button missing or a message saying you cannot.
- **Typing indicators.** The server announces when somebody is typing. Nothing
  draws it.
- **Editing a message.**
- **Channel categories**, for grouping channels under headings.
- **Invite limits** — uses and expiry.
- **A member list** for a server. The app knows who is in one; it only uses that
  inside calls.

---

## 10. Things that do not exist at all

These need server work before they can be designed as anything real:

- Avatars and server pictures, and uploading them
- Attachments — images and files in messages
- Any notification, in-app or from the operating system
- A settings screen of any kind, including choosing a microphone
- Profiles — clicking a name does nothing
- Search, pins, replies, reactions, threads
- Direct messages
- Recovering a forgotten password
- Statuses beyond online and offline
