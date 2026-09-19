# Split Bot WhatsApp Ingress

WhatsApp ingress (whatsmeow) that routes chats by mode to SplitBot, Nanobot, or Hermes.

## Modes

| Mode | Behavior |
|------|----------|
| `silent` | Ignore messages |
| `splitbot` | Sync AI via SplitBot |
| `nanobot` | Forward to Nanobot `POST /message`; async reply via `/send_message_to_chat` |
| `hermes` | Forward to Hermes `POST /message` (text + images + documents, including captioned files); async reply via `/send_message_to_chat` / `/send_media_to_chat` |
| `playground` | Local playground handler |

Admin: `/onboard hermes` or `/whitelist-group <jid> hermes`.

## Env

```bash
DATABASE_URL=postgres://...
SPLIT_BOT_URL=http://split-bot:8000
NANOBOT_URL=http://nanobot:8000
HERMES_URL=http://hermes:8765
HERMES_API_KEY=optional-shared-secret
BOT_NAME=YourBot
PORT=8080
GOOGLE_SPREADSHEET_ID=your-spreadsheet-id
GOOGLE_SERVICE_ACCOUNT_JSON={"type":"service_account",...}
# or GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

Google Sheets export writes a new tab into an existing spreadsheet you own. Share that spreadsheet with the service account `client_email` as Editor, and set "anyone with the link" if the group should open it.

Hermes must set `WHATSAPP_INTERNAL_API_URL` to this service (e.g. `http://split-bot-whatsapp:8080`) and the same API key.

## Egress HTTP API (called by Nanobot / Hermes)

- `POST /send_message_to_chat` — `{ "message", "chat_id" }`
- `POST /send_media_to_chat` — `{ "chat_id", "media_type", "data_base64"|"file_url", "mime?", "caption?", "filename?", "reply_to?" }`
- `POST /typing` — `{ "chat_id" }`
