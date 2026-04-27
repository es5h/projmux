# Terminal Keybindings

`projmux shell` and `projmux tmux install` generate the tmux bindings used by the
app. In many terminals, `Alt-1`, `Alt-2`, and the other default shortcuts pass
through to tmux without extra setup.

Use this page when your terminal emulator consumes a shortcut before tmux sees
it, or when you want explicit terminal-level bindings. The examples send CSI-u
escape sequences that projmux maps to tmux `User0` through `User11` keys.

한국어 요약: 보통은 별도 설정 없이 `projmux shell`이 생성한 tmux 키가 동작합니다.
터미널이 `Alt-1` 같은 조합을 먼저 가로채면 아래 Ghostty 또는 Windows Terminal
설정을 추가해서 projmux 전용 CSI-u 시퀀스를 tmux로 보내면 됩니다.

## Generated App Bindings

| Shortcut | Action |
| --- | --- |
| `Alt-1` | Open project sidebar |
| `Alt-2` | Open existing session popup |
| `Alt-3` | Open project switcher popup |
| `Alt-4` | Open AI split picker |
| `Alt-5` | Open settings |
| `Ctrl-n` | New tmux window in the current pane directory |
| `Alt-r` | Rename the current tmux window |
| `Ctrl-M` | Rename the current tmux window when your terminal sends `User10` |
| `Alt-Left/Right/Up/Down` | Move between panes |
| `Alt-Shift-Left/Right` | Previous/next window |
| `Ctrl-Shift-M` | Rename the current tmux pane label when your terminal sends `User11` |
| `Prefix b` | Existing session popup |
| `Prefix f` | Project switcher popup |
| `Prefix F` | Project sidebar |
| `Prefix g` | Jump to the current pane project session |
| `Prefix r` | Open AI split to the right |
| `Prefix l` | Open AI split below |
| `Prefix R` | Rename the current tmux window |

When a pane exits or is killed, projmux asks tmux shortly after pane removal to
spread panes evenly in every multi-pane window so the previous split does not
leave one side oversized.

## Picker Actions

| Surface | Shortcut | Action |
| --- | --- | --- |
| Existing session popup | `Ctrl-X` | Kill the focused session and reopen the popup |
| Existing session popup | `Left/Right` | Preview previous/next window |
| Existing session popup | `Alt-Up/Alt-Down` | Preview previous/next pane |
| Project switcher | `Ctrl-X` | Kill the focused existing session and reopen the picker |
| Project switcher | `Alt-P` | Pin or unpin the focused directory |

## CSI-u Map

| CSI-u sequence | tmux key | Action |
| --- | --- | --- |
| `ESC [ 9001 u` | `User0` | Open AI split to the right |
| `ESC [ 9002 u` | `User1` | Open AI split below |
| `ESC [ 9003 u` | `User2` | Existing session popup |
| `ESC [ 9004 u` | `User3` | Project switcher popup |
| `ESC [ 9005 u` | `User4` | Project sidebar |
| `ESC [ 9006 u` | `User5` | AI split picker |
| `ESC [ 9007 u` | `User6` | Settings |
| `ESC [ 9008 u` | `User7` | New tmux window in the current pane directory |
| `ESC [ 9009 u` | `User8` | Previous tmux window |
| `ESC [ 9010 u` | `User9` | Next tmux window |
| `ESC [ 9011 u` | `User10` | Rename the current tmux window |
| `ESC [ 9012 u` | `User11` | Rename the current tmux pane label |

## Ghostty

Add key bindings to your Ghostty config. Ghostty keybinds use
`keybind = trigger=action`; the `csi:` action sends a CSI sequence without the
leading `ESC [` bytes.

```text
keybind = alt+1=csi:9005u
keybind = alt+2=csi:9003u
keybind = alt+3=csi:9004u
keybind = alt+4=csi:9006u
keybind = alt+5=csi:9007u

keybind = ctrl+shift+r=csi:9001u
keybind = ctrl+shift+l=csi:9002u

keybind = ctrl+shift+n=csi:9008u
keybind = ctrl+m=csi:9011u
keybind = ctrl+shift+m=csi:9012u
keybind = alt+shift+left=csi:9009u
keybind = alt+shift+right=csi:9010u
```

Reload Ghostty or restart the terminal after changing the config.

## Windows Terminal

Add `sendInput` actions to `settings.json` and bind them from `keybindings`.
Windows Terminal works well with plain tmux escape sequences for the default
`Alt` shortcuts, while the split actions can send tmux prefix sequences
directly. Escape bytes should be written as `\u001b`.

```json
{
  "actions": [
    { "command": { "action": "sendInput", "input": "\u001b1" }, "id": "User.projmuxSidebar" },
    { "command": { "action": "sendInput", "input": "\u001b2" }, "id": "User.projmuxSessions" },
    { "command": { "action": "sendInput", "input": "\u001b3" }, "id": "User.projmuxSwitch" },
    { "command": { "action": "sendInput", "input": "\u001b4" }, "id": "User.projmuxAIPicker" },
    { "command": { "action": "sendInput", "input": "\u001b5" }, "id": "User.projmuxSettings" },
    { "command": { "action": "sendInput", "input": "\u0002r" }, "id": "User.projmuxAISplitRight" },
    { "command": { "action": "sendInput", "input": "\u0002l" }, "id": "User.projmuxAISplitDown" },
    { "command": { "action": "sendInput", "input": "\u000e" }, "id": "User.projmuxNewWindow" },
    { "command": { "action": "sendInput", "input": "\u001b[1;4D" }, "id": "User.projmuxPrevWindow" },
    { "command": { "action": "sendInput", "input": "\u001b[1;4C" }, "id": "User.projmuxNextWindow" },
    { "command": { "action": "sendInput", "input": "\u001b[9011u" }, "id": "User.projmuxRenameWindow" },
    { "command": { "action": "sendInput", "input": "\u001b[9012u" }, "id": "User.projmuxRenamePane" }
  ],
  "keybindings": [
    { "id": "User.projmuxSidebar", "keys": "alt+1" },
    { "id": "User.projmuxSessions", "keys": "alt+2" },
    { "id": "User.projmuxSwitch", "keys": "alt+3" },
    { "id": "User.projmuxAIPicker", "keys": "alt+4" },
    { "id": "User.projmuxSettings", "keys": "alt+5" },
    { "id": "User.projmuxAISplitRight", "keys": "ctrl+shift+r" },
    { "id": "User.projmuxAISplitDown", "keys": "ctrl+shift+l" },
    { "id": "User.projmuxNewWindow", "keys": "ctrl+n" },
    { "id": "User.projmuxPrevWindow", "keys": "alt+shift+left" },
    { "id": "User.projmuxNextWindow", "keys": "alt+shift+right" },
    { "id": "User.projmuxRenameWindow", "keys": "ctrl+m" },
    { "id": "User.projmuxRenamePane", "keys": "ctrl+shift+m" }
  ]
}
```

This example matches the default projmux app shortcuts without depending on
CSI-u support from Windows Terminal for the `Alt` shortcuts. `Ctrl-M` and
`Ctrl-Shift-M` still use CSI-u so tmux can distinguish rename commands from
plain Enter. If a key is already bound by Windows Terminal, remove or change
the conflicting `keybindings` entry before adding the projmux binding.
