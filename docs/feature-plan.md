# Feature Plan

This document tracks planned projmux feature and behavior work before
implementation branches are opened or merged. Each row should stay small enough
to land as one branch.

## Planned Branches

| Status | Branch | Scope | Notes |
| --- | --- | --- | --- |
| Pending | `fix/hide-preview-window-badge` | Hide window aggregate badges in bottom/session preview window rows. | Keep pane badges visible; keep session/sidebar row aggregate badge. A draft worktree exists but is not merged. |
| Pending | `fix/attention-pane-focus-only-clear` | Verify green reply badges clear only on actual pane focus. | Add regression coverage around session/sidebar navigation not clearing pane state. |
| Pending | `fix/session-window-attention-aggregation` | Keep session and window badges derived only from child panes. | Session badge aggregates all panes in the session; window badge aggregates panes in that window where visible. |
| Pending | `feat/notification-payload-json` | Add optional JSON stdin payload for `PROJMUX_NOTIFY_HOOK`. | Preserve current positional hook args for compatibility. |
| Pending | `feat/notification-preview-snippet` | Improve notification copy with a safe answer/session snippet. | Prefer pane topic/title; avoid noisy tmux pane ids and raw status lines. |
| Pending | `fix/ai-watcher-lifecycle` | Harden watcher cleanup for closed panes and split lifecycle edge cases. | Existing fix stops empty pane-id lookups; add broader regression coverage. |
| Pending | `feat/sessionizer-preview-polish` | Keep sessionizer/sidebar preview labels user-facing. | Avoid internal terms; keep pane/window status concise. |

## Branch Flow

1. Create or reuse a task worktree with `wt path --create <branch>`.
2. Keep each branch scoped to one row above.
3. Run `make fmt`, `make fix`, `make test`, `make test-integration`, and `make test-e2e`.
4. Merge to `main`.
5. Push `main`.
6. Apply locally by installing `/home/es5h/.local/bin/projmux` and reloading current tmux config/watchers when the change affects runtime behavior.

## Badge Semantics

- Pane badge is the source of truth.
- Window badge, when shown, is an aggregation of panes in that window.
- Session badge is an aggregation of panes in that session.
- Green reply badges clear only when the relevant pane receives focus.
- Sidebar/session preview navigation must not clear pane reply state by itself.
