---
name: review-and-fix
description: Spawn caveman reviewer subagent, then fixes all issues this subagent finds.
---

Spawn an AI subagent with `caveman-review` skill activated (do not follow it yourself). Let it review the code, next it report all issues to you. After that, you investigate every reported issue and decide whether it is really need fixing or should be dismissed (e.g. reviewer was wrong and the problem is not real, not significant or will bring other regressions that will make only worse).
If you need my decision for any of reported issues - ask me, proposing solution and specifying which one you recommend.
