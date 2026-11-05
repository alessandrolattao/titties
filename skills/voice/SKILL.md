---
name: voice
description: Change the language or voice TitTieS uses to call the user out loud. Use when the user wants another voice, another language, or to hear the available voices.
---

# Change the TitTieS voice

The voice is stored in `~/.config/titties/config.json` and shared by the plugin and the `titties` command.

- **The user wants to choose by ear:** tell them to run `titties voice` in their own terminal. It is interactive (language first, then only that language's voices, each one says "Hello, I am <name>" in its language before saving), so it cannot run through the Bash tool or `!`.
- **The user already names a voice or a language:** set it for them with the Bash tool:
  - `titties list -lang <language>` shows the voices of a language (`it`, `en_US`, ...);
  - `titties default -lang <language> -voice <name>` saves it; `-voice auto` picks a good one for the language;
  - `titties say "<a short sentence>"` lets them hear it.

If `titties` is not found, the plugin has not finished its first install yet: it happens in the background at the start of a session; ask the user to start a new session.
