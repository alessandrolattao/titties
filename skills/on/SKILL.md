---
name: on
description: Turn on TitTieS voice notifications for this session only. Every other session stays silent unless it is turned on too.
disable-model-invocation: true
---

!`sh "${CLAUDE_PLUGIN_ROOT}/hooks/session.sh" on "${CLAUDE_SESSION_ID}" "${CLAUDE_PLUGIN_DATA}"`

Tell the user, in one line, the result above. Do nothing else.
