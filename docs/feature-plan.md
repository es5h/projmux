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
| Pending | `feat/settings-info-tab` | Add an Info tab/section to settings. | Include app version, GitHub repository URL, install/update hint, and generated keybinding overview. |
| Pending | `docs/user-keybinding-guide` | Document terminal-level user key bindings for projmux. | Explain that projmux abstracts tmux bindings through tmux `UserN` keys; users wire Ghostty/Windows Terminal to send CSI-u sequences. |
| Pending | `docs/ghostty-rename-key` | Add Ghostty rename-window binding guidance. | Recommend `ctrl+m` sending the rename-window user key sequence. Check conflict risk with Enter/C-m before implementation docs land. |
| Pending | `fix/shell-pane-title-policy` | Use shell process names for plain shell pane titles. | Shell panes should show `zsh`, `bash`, etc. instead of branch-derived titles. Keep AI pane titles agent/topic based. |
| Pending | `fix/panes-preview-agent-label` | Show AI agent names in Alt-2 panes preview instead of raw process names. | For AI panes show `codex`, `claude`, etc.; for normal panes keep process name plus pane number/title/status metadata. |
| Pending | `fix/popup-minimum-dimensions` | Add minimum dimensions for sidebar, popup, and preview surfaces. | Keep percentage sizing, but enforce minimum width/height so required information remains visible. |

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

## Settings Info Tab Requirements

- Add a top-level settings entry for app information.
- Show `projmux` version from the existing version command/source.
- Show GitHub repository: `https://github.com/es5h/projmux`.
- Show concise install/update guidance without replacing the README.
- Show keybinding guidance as an overview, not a full terminal manual.
- Include the tmux generated app bindings and the terminal user-key layer:
  - projmux binds tmux `User0` through `User10`.
  - terminal emulators should send the configured CSI-u escape sequences.
  - Ghostty and Windows Terminal should have copyable examples.

## Keybinding Guide Requirements

- Keep the app-level action names stable:
  - project sidebar
  - existing session popup
  - project switcher
  - AI split picker/settings
  - new window
  - previous/next window
  - rename current window
- Document Ghostty examples.
- Document Windows Terminal `sendInput` examples.
- For Ghostty, prefer `ctrl+m` for rename-window if it does not conflict with
  Enter/C-m behavior in the target terminal stack; otherwise call out the
  conflict and keep the existing fallback binding.

## Preview Display Requirements

- Plain shell pane title should be the shell/process name (`zsh`, `bash`, etc.).
- AI pane title should continue to reflect agent/topic/status.
- Alt-2 panes preview should show:
  - pane number
  - pane title
  - agent name for AI panes, otherwise process name
  - readable status metadata

## Popup Sizing Requirements

- Keep current percentage-based sizing as the primary layout behavior.
- Add minimum dimensions per surface where tmux supports it or through computed
  popup options:
  - sidebar
  - session popup
  - project switcher popup
  - AI split picker/settings
  - preview panes
- Minimum sizes should preserve at least:
  - item name/title
  - session/window/pane identity
  - key status badge
  - one useful metadata column
- Avoid expanding beyond the terminal size; clamp safely on small terminals.
