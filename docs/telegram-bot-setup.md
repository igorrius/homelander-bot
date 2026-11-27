# Setting up the Telegram bot for chats and labels

Homelander keeps a list of the groups, forums and topics its bot is in (admin panel → **Chats**).
You hang **labels** on them — "journal", "residents", … — and the system writes to a label, not to a
chat ID. This guide covers the bot settings that make that work. The bot creation itself is in
[Deploying on Railway](deploy-railway.md), step 1.

## 1. Privacy mode off

In @BotFather: `/setprivacy` → choose the bot → **Disable**.

With privacy mode on, Telegram shows the bot only commands and replies to it, so it cannot see the
messages that reveal a chat or its forum topics.

**The change only applies to groups the bot joins afterwards.** If the bot was already in a group
when you turned privacy off, remove it from that group and add it again, or make it an
administrator there (an administrator bot sees every message regardless of privacy mode).

## 2. Who may add the bot to groups

Anyone can add a bot to a group. Homelander protects itself in two ways:

- a chat the bot was added to by an **active admin of the system** (with Telegram linked) is
  accepted at once;
- a chat the bot was added to by **anyone else** is left automatically after two minutes, unless an
  admin accepts it in **Chats** first. (The pause exists because turning on topics in a group gives it
  a new ID, and Telegram may report the bot as "added" to the new ID by whoever turned topics on.)

For a second lock, BotFather's `/setjoingroups` controls whether the bot can be added to groups at
all. Enable it while you add the bot to the system's groups, then **Disable** it again. While it
is disabled nobody, you included, can add the bot to a group.

If a group admin adds the bot while "remain anonymous" is on, Telegram does not say who did it; the
chat then waits in **Chats** as **new** until an admin accepts it.

## 3. Rights per chat

| Chat | What the bot needs |
|---|---|
| Admins' journal (notices, later gate logs) | A regular member is enough: it only writes there. |
| Community group or forum | Make it an **administrator with no extra rights**. It then sees every message whatever the privacy mode, and later milestones will need the join/leave events only administrators get. |
| Telegram channel | An administrator with "Post messages". The bot only posts; it does not read channels. |

If the bot is restricted so that it cannot send, **Chats** shows a warning on that chat.

## 4. Check that it works

1. As an admin of the system, add the bot to a test group. It appears in **Chats** as **accepted**.
2. In a group the bot was already in, write any message. The group appears as **new**: accept it or
   make the bot leave.
3. In a forum, write in a topic. The topic appears under its forum; a topic the bot has never seen
   created shows as "Topic #N (no name yet)" until someone writes a reply in it or renames it. You
   can also name it yourself.
4. Create a label, hang it on a chat or topic, and press **Send test**. The result shows next to
   the button: delivered, or the reason it was not.
5. In **Settings → Notifications**, pick the label for **New applications**. Each new application
   is announced there (name, house, apartment, status; never the phone number).

## 5. A group the bot was in before this version

Before chats and labels existed, the bot did not record the groups it was added to. For each such
group (for example the admins' journal):

- either write a message there and accept the group in **Chats**;
- or remove the bot from the group and add it again as an admin of the system — it is accepted
  at once, and step 1's privacy caveat is taken care of at the same time.

## Registration from the bot

Nothing to configure. A person who shares their phone with the bot and is not registered gets a
one-hour link to the registration form with the number already filled in. After an administrator
approves the application, the person can log in at once; the bot tells them about the decision.
Links in bot messages are buttons when `PUBLIC_URL` is `https`, plain text otherwise (local
development).
