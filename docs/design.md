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
| **Picture** | Optional. Uploaded, cropped to a square and served at 256×256. Most people will not have one |

**Status is only two values.** Idle, do-not-disturb, invisible, custom status,
"playing something" — none of these exist. If any of them should, that is new
work on the server, not just a design.

**A picture is the exception, not the rule.** Everybody has a name; hardly
anybody will have uploaded a picture, and a server that nobody gave an icon to
is the normal case. **Both fall back to the first two characters of the name,
uppercased** — which is a design decision currently made in code and worth
making properly, because it is what most of the app will actually show. A blank
name falls back to `?`, and an image that fails to load falls back to the same
letters rather than to a broken picture.

Pictures are square and cannot be anything else: the server crops the middle of
whatever arrives and scales it to 256×256, so a wide photo loses its sides
without asking. Nothing shows the person what will be kept before it happens.
There is no cropper and no preview of the crop, and offering one is a design
decision with real server work behind it.

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

**A call is the one place a picture is not shown.** Messages, the member list, a
quoted reply and the server list all carry one; the people in a call are names
alone, though the data is there and nothing prevents it. Anyone designing a call
around faces is designing something the app can already supply.

### Deafening, and how it differs from muting

**Whether they can hear you** is now a state of its own. Muting is "I am not
talking"; deafening is "I am not listening", and the two are not symmetrical:
deafening takes the microphone down with it, because somebody who cannot hear
the room should not be broadcasting into it. Undoing it comes back to a live
microphone, so the pair is one control, not two independent ones.

The server enforces it rather than trusting the client to stay quiet: a
deafened member is sent no voice and no screen sound at all. **The picture is
deliberately still sent** — deafening is about hearing, and somebody who cannot
hear can still watch a screen being shared. So a deafened viewer is a real
state to design: silent, still watching.

### Their connection quality

Three grades, and **the good one is deliberately silent**. A green light beside
every name is noise; the interesting cases are the two that are not fine:

| Grade | What it means | What people would notice |
| --- | --- | --- |
| good | Under 2% loss and under 200ms round trip | Nothing, and it says nothing |
| weak | 2–8% loss, or 200–400ms | Occasional clipped words |
| bad | Over 8% loss, or over 400ms | Words breaking up, people talking over each other |

It arrives about every five seconds and only when it changes, so it is a state
that settles rather than a number that flickers. It is a fact about **their**
link, the same for everybody in the call, so it belongs next to their name and
not next to yours.

Still missing, and still needing server work: nothing tells you your **own**
connection is the bad one. Everybody sees everybody else's grade.

### Who is in a voice channel before you join

**You can see this now.** A call in progress is visible from the channel list
without joining it: who is in it, who is muted, and who cannot hear. It survives
opening the app mid-call and joining a server mid-call, which is what it did not
do before.

Two things follow for a design. A voice channel is a **container with people in
it**, not a link — it can be one line tall or five depending on who is in it,
and it changes while somebody is looking at it. And a voice channel nobody can
see reports nothing, so an empty-looking channel list is not proof the server is
quiet.

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

**Choosing a picture** — for yourself, or for a server you may manage. Four
states: none yet, where the control reads **Add** · one set, where it reads
**Change** and a **Remove** appears beside it · uploading, where every control
is disabled and nothing else says it is working · refused, with a message under
it. A server's icon is chosen inside its settings, which is a sensible home;
your own has none, so it sits loose in the bar beside your name and the way out,
purely because there is nowhere else to put it. That bar is also where the app
says whether it is connected, and — only when it has been pointed at some server
other than the one it was built for — which address it is talking to. Refusals come from the server *after* the whole file has been sent, because
nothing is checked in the browser first — so the slowest possible failure is
also the most likely one, and it is worth designing for. What gets refused: not
an image we can read, larger than 5 MB, or more than 24 megapixels however
small the file is.

**Files on a message** — the composer stages them first: chosen files sit above
it as chips with a way to take each one back, up to ten, and they are sent with
the message rather than before it. Once sent, an image shows inline and opens
in a new tab when clicked; anything else is a download chip with a filename and
a size.

**An upload now shows itself.** The staged chips are replaced by a bar while
the files go up, and past 100% it says "almost there" rather than sitting full:
the bar measures bytes leaving the browser, and a picture still has re-encoding
to do after that. So there are two waits, and only the first one has a length.

Two gaps left here, both things people will simply try:

- **No dragging a file onto the window, and no pasting a screenshot.** Only the
  attach button opens the picker.
- **No lightbox.** An image opens as a new browser tab, which on the desktop app
  means leaving the app.

And one thing that is not a gap but will look like a bug: **a link to a file
expires a day after the app asked for it.** Pictures on a message are private to
the channel, so their addresses are signed and time-limited, which means a copied
image address is useless to somebody outside — and a window left open for a day
shows broken pictures until it reloads. Pictures of people and servers work the
other way: no expiry, no sign-in needed, an unguessable address, because a
picture beside a name has to load in an ordinary `<img>` tag.

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

**Making a server** — a name, 1–100 characters. Nothing else: no description
and no template, and **no picture at this point** — an icon can only be added
afterwards, in the server's settings, by somebody who may manage it. So every
server is born with letters where its icon goes.

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

**Ordering channels** — the order is now a decision rather than an accident of
which channel was made first, and it holds for everybody. Whoever may manage
channels gets two arrows on hover; everyone else sees nothing and simply gets
the order. **Moving a channel into another category is not possible**, only
moving it within the one it is in, so the design of that is still open — it is
the drag interaction nobody has built.

Worth knowing for a design: a move is announced per channel, and a channel
somebody cannot see is not announced to them at all, so two people can hold
different pictures of the same list without either being wrong.

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
| A picture for a person or a server | 5 MB, and 24 megapixels however small the file |
| What a picture becomes | A 256×256 square, cropped from the middle |
| A link to a file on a message | Stops working 24 hours after it was made |
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

---

## 10. Things that do not exist at all

These need server work before they can be designed as anything real:

- Any notification, in-app or from the operating system
- Push-to-talk, and any global hotkey. That one is not a browser feature at all:
  a key that works while the app is not focused needs the desktop shell
- A settings screen of any kind. There is now one thing about yourself you can
  change — your picture — and no screen for it to live on, so it is pinned to
  the chrome next to your name. **Choosing a microphone and joining muted exist
  but live beside the call**, in a panel behind an "Audio" button, for the same
  reason: they are preferences with nowhere to live. They are kept in the
  browser rather than on the account, so they do not follow somebody to another
  machine, and the panel says a change applies the next time you join
- Profiles — clicking a name does nothing
- An emoji picker worth the name, and custom per-server emoji
- Search, pins, threads
- Direct messages
- Recovering a forgotten password
- Statuses beyond online and offline
