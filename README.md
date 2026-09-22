# mtproto-mcp

[![ci](https://github.com/r4nol/mtproto-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/r4nol/mtproto-mcp/actions/workflows/ci.yml)

MCP server that exposes a Telegram **userbot** (a real user account over MTProto, not the Bot API) as tools for Claude and other MCP clients.

Single static binary. Built on [gotd/td](https://github.com/gotd/td) (MTProto) + [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) (MCP over stdio).

## Tools

| Tool | Purpose |
|------|---------|
| `auth_status` / `auth_send_code` / `auth_sign_in` / `auth_password` | Interactive login flow |
| `get_me` | Logged-in account info |
| `resolve_peer` | `@username` / phone / t.me link / `me` → peer |
| `list_dialogs` | Recent chats with peer ids and unread counts |
| `get_history` | Messages in a chat, newest first (`offset_id` for paging) |
| `search_messages` | Search within one chat |
| `search_global` | Search across all chats |
| `send_message` | Send text, optionally as a reply (`reply_to`) |
| `edit_message` | Edit a message you sent |
| `delete_messages` | Delete messages (for everyone by default) |
| `forward_messages` | Forward messages between chats |
| `send_reaction` | React to a message with an emoji |
| `mark_read` | Mark a chat as read |
| `send_file` | Upload and send a local file (document or photo) |
| `download_media` | Save a message's photo/document to disk |

**Peers.** Anywhere a `peer` is taken you can pass `@username`, a phone number, a `t.me` link, `me`, or the `user:ID` / `chat:ID` / `channel:ID` strings returned by `list_dialogs` and `resolve_peer`. Numeric peers need their access hash, which is cached in memory after `list_dialogs`, `get_history` or a search returns them — call `list_dialogs` first after a restart.

Messages come back as JSON with `id`, `date`, `out`, `from`, `chat`, `text`, and when present `reply_to` and `media`.

## Install

Download a binary from [Releases](https://github.com/r4nol/mtproto-mcp/releases), or:

```bash
go install github.com/r4nol/mtproto-mcp@latest
```

## Setup

1. Get `api_id` + `api_hash` from https://my.telegram.org → API development tools.
2. Configure your MCP client (Claude Desktop, Claude Code, …):

```json
{
  "mcpServers": {
    "telegram": {
      "command": "/absolute/path/to/mtproto-mcp",
      "env": {
        "TG_API_ID": "1234567",
        "TG_API_HASH": "abcdef0123456789abcdef0123456789",
        "TG_SESSION": "/absolute/path/.mtproto-mcp.session.json"
      }
    }
  }
}
```

Claude Code one-liner:

```bash
claude mcp add telegram -e TG_API_ID=1234567 -e TG_API_HASH=abcdef... -- /absolute/path/to/mtproto-mcp
```

| Env | Required | Default |
|-----|----------|---------|
| `TG_API_ID` | yes | |
| `TG_API_HASH` | yes | |
| `TG_SESSION` | no | `~/.mtproto-mcp.session.json` |
| `TG_PHONE` | no | prompted by `login` |

### Login

The server starts unauthenticated and exposes login as tools, so no separate terminal is needed. Ask your client to:

1. `auth_status` → check state
2. `auth_send_code` `{ "phone": "+15551234567" }` → Telegram sends a code
3. `auth_sign_in` `{ "code": "12345" }`
4. if 2FA is enabled: `auth_password` `{ "password": "…" }`

The session persists to `TG_SESSION`, so later runs are already authorized. Other tools return a "not logged in" error until login completes.

Or log in from a terminal (same session file, keeps your 2FA password out of the chat transcript):

```bash
TG_API_ID=... TG_API_HASH=... mtproto-mcp login
```

## Build

```bash
go build -o mtproto-mcp .
go test ./...
```

Releases are built by GoReleaser when a `v*` tag is pushed.

## Security

- The session file grants **full access to your Telegram account**. Keep it private (it's in `.gitignore`).
- An MCP client with these tools can read, send, and delete messages as you. Review tool calls before approving them, especially `delete_messages`, `forward_messages`, and `send_file` (it can upload any local file the server can read).
- Userbot automation can violate Telegram's ToS if abused (spam, scraping). Use responsibly.

## License

MIT
