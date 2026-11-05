---
name: call
description: Calls the user out loud through TitTieS with one sentence, when a skill such as release-manager needs them for a decision or a block. Works only in a session where TitTieS is on; never for progress or good news.
---

# Call the user out loud

Run this with the Bash tool, putting your sentence in place of the second line and nothing else:

```sh
sh "${CLAUDE_PLUGIN_ROOT}/hooks/call.sh" "${CLAUDE_PLUGIN_ROOT}" "${CLAUDE_PLUGIN_DATA}" "${CLAUDE_SESSION_ID}" <<'TITTIES'
Il release-manager di my-app ha una domanda sulla issue 42: quale tabella usa il nuovo endpoint.
TITTIES
```

The sentence:

- one sentence, two at most, in the user's language;
- says who calls (the role and the repo) and what for, so the user knows which session to open;
- plain words only: no markdown, no URLs, no code, no branch names spelled out;
- only when the user is needed (a decision, a block, a question waiting on an issue), never for progress.

The script answers "TitTieS is off for this session" when the session was not turned on: then say it in the conversation instead, and do not retry.
