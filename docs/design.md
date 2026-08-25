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

The screen itself, **whose it is**, and a choice of how much of it to receive:
**full**, **smaller**, or **stop**. All three are real — the server sends only
what was asked for, so choosing smaller genuinely costs less rather than shrinking
a picture that arrived anyway. A stopped share can be taken back at any time, and
a viewer who chose smaller keeps that choice when they do.

Not offered: making a share fullscreen or popping it out. And the sizes are the
sharer's two, not a slider — the publisher decides what exists, the viewer picks
from it.

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

**Somebody in the member list** — online or offline, grouped and counted
separately. Offline is shown rather than hidden: in a group of friends, knowing
somebody exists matters more than knowing they are out.

**Somebody who joins while you are looking appears straight away**, without a
reload.

**A channel in the list** — normal · the one you are reading · **has something
unread**. Channels may sit under a category heading or loose above them. Unread is a yes/no; the server does not say how many, so a number
badge would need new work.

**Messages** — loading · none yet · older ones available to load · failed to
send · being edited. Messages are plain text, up to 4000 characters, at 60 per
minute, and may carry up to ten files. No threads.

**A message answering another** — the quoted parent sits above it, one level
only, cut at 120 characters with a mark for a file it carried. Three states, and
the third is the one to get right: quoted normally · **the parent deleted**,
which shows "Original message was deleted" and deliberately carries neither the
text nor who wrote it · the parent too far back to have been loaded, where
clicking the quote cannot jump anywhere and currently says so as an error.

**Answering something** — the composer grows a bar naming who you are replying
to, with a way out of it; Escape also cancels. A message that answers something
is never grouped under the one above it, because the quote needs a name over it
to make sense.

**A reaction on a message** — none · several · one of them yours. A reaction is
a count with a yes/no for whether you are in it, so a chip has two states rather
than one, and the whole row appears and disappears as people add and take back.
The names behind a count are fetched only when somebody asks for them, so
"who reacted" has a moment of not knowing yet.

**Adding a reaction** is currently a fixed row of eight emoji, because a real
picker — search, categories, skin tones, recently used — is a design problem of
its own and the server accepts any Unicode emoji, not only those eight. Somebody
without the Add reactions permission still sees the counts, and can still take
back one of their own.

**Somebody typing** — one person, several people, nobody. It arrives as a
moment rather than a state, so it fades on its own after a few seconds rather
than being taken back.

**Making a server** — a name, 1–100 characters. Nothing else: no picture, no
description, no template.

**Managing a server** — roles, who has them, and what each role may do in each
channel. A role has a name, a position that decides what it outranks, and
fourteen permissions. Per channel, each permission can be **allowed, denied, or
left to inherit** from the role itself, which is three states rather than two and
is the hardest thing here to make legible. Deny beats allow. The everyone role
cannot be renamed, moved or removed, and cannot be taken away from anybody.

Every one of these actions can be refused: you cannot edit a role that outranks
you, and you cannot grant a permission you do not hold. Those two refusals still
arrive as a message after the attempt, because they depend on which role you are
editing rather than on what you hold.

**Everything else the app now knows in advance.** A member is told what they may
do, per channel and per server, when they connect and again the moment it
changes. So controls they cannot use are not offered at all: no settings gear
without Manage roles, no new-channel button without Manage channels, no invite
box without Create invites, and a composer that says so rather than failing on
send.

**Making a channel** — a name and one of three kinds: text, voice, or a category
to group the other two under. Needs the `ManageChannels` permission, which the
default role does not carry, so for most members the control is simply absent.

**Joining a server** — by pasted code, or by opening a link.

**Inviting someone** — produces a link to copy, optionally limited by number of
uses and by expiry, both blank by default. Existing invites can be listed and
revoked, which is its own small state: none yet · several · one being revoked.

**Removing somebody** — a kick or a ban, both reached from the member list and
offered only to people who hold the matching permission, on people they outrank.
Neither is offered on yourself or on the owner, who cannot be removed at all. A
kick lets them return with a new invite; a ban does not, until it is lifted. Both
take effect at once: the app they are using loses the server as they are looking
at it, and drops them out of any call. What either of them wrote stays where it
is, still shown as theirs — worth saying in the dialog, because people assume a
ban erases the argument that caused it.

A ban carries an optional reason of up to 500 characters and remembers who set
it. **Bans are listed and lifted** in server settings, behind Ban members — the
list is its own small state: nobody banned · several · one being lifted. Without
it a ban is permanent by accident.

The honest limit, worth stating where somebody bans: a ban is by account, and
nothing here verifies an email address. Somebody determined can register again in
seconds and use a fresh invite.

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
| Files on a message | 10, of 25 MB each |
| Kinds of reaction on a message | 20 |
| Quoted reply preview | 120 characters, one level |
| Reactions added or taken back | 60 per minute |
| Sign-in attempts | 20 per minute per account |
| Accounts created | 10 per hour |
| Devices signed in at once | 5 |
| Volume range | 0–200% |
| Going offline | ~90 seconds after closing the app |

---

## 9. Things that exist but nobody can see

Built and working on the server, with no interface at all. Any of these is
available to design without new server work:

- **Overwrites aimed at one person** rather than a role. The server accepts
  either; the settings dialog only offers roles.
- **Who is in a voice channel, before you join it.** The server names people in a
  call only once you are in it, and announces arrivals only while you are
  watching. So the app knows who is in a call it has joined, and nothing about
  one it has not. Showing faces under every voice channel — what people expect —
  needs server work first.

---

## 10. Things that do not exist at all

These need server work before they can be designed as anything real:

- Any notification, in-app or from the operating system
- A settings screen of any kind, including choosing a microphone
- Profiles — clicking a name does nothing
- An emoji picker worth the name, and custom per-server emoji
- Search, pins, threads
- Direct messages
- Recovering a forgotten password
- Statuses beyond online and offline
