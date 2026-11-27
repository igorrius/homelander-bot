# Connecting an AI agent through MCP

Homelander has an [MCP](https://modelcontextprotocol.io) server at `<PUBLIC_URL>/mcp`. An admin can
connect an AI agent such as Claude to it; the agent then reads and changes the **data space** and
writes **drafts of the home page**. It never publishes: an admin previews a draft and publishes it in
the panel (**Admin → Pages**).

Everything the agent does is done as the admin who connected it, and the audit log shows it as
"*admin* · via MCP (*agent*)".

## Who can connect an agent

Only a user with both **Admin** and **MCP** turned on. Any admin can switch MCP on or off for any
admin, including themselves, on the user card (**Admin → Users → the user → Roles → MCP**). The
switch has no effect without Admin.

Turning MCP or Admin off, or disabling the user, cuts every connected agent off at its next request.

## Connect Claude

1. Turn MCP on on your own user card.
2. In Claude (claude.ai, Claude Desktop or the mobile app): **Settings → Connectors → Add custom
   connector**. Name it, and set the URL to `<PUBLIC_URL>/mcp` (for example
   `https://home.example.org/mcp`). No client ID or secret is needed.
3. Claude opens a Homelander page that asks to log in (phone + code in Telegram, as usual) and then
   shows **"Allow Claude to act as you in Homelander?"** with the address the answer goes to. Check
   that it is `claude.ai` (or `claude.com`), then press **Allow**.
4. Ask Claude to look at the data (`data_catalog`) or to design the home page. It reads the page
   guide first, writes a draft and gives you a link like `<PUBLIC_URL>/admin/pages/12` to preview
   and publish.

Claude Code works the same way (`claude mcp add --transport http homelander <PUBLIC_URL>/mcp`); it
uses a loopback address (`http://localhost:<port>/…`) for the answer.

## See and revoke connected agents

The user card shows **Channels → Connected agents**: the agent's name, when it was connected and
last used. **Revoke** disconnects it at once (its tokens stop working); connecting again needs a new
approval.

## Other agents

To keep strangers from registering a fake "Claude" that sends approvals to their own server,
Homelander accepts only agents whose answer address is on an allow-list. The default list is:

```
https://claude.ai/api/mcp/auth_callback
https://claude.com/api/mcp/auth_callback
http://localhost:*
http://127.0.0.1:*
```

`*` stands for any port and is allowed only for `localhost` and `127.0.0.1` (local tools such as
Claude Code). To allow another agent, set `MCP_REDIRECT_URIS` to the full comma-separated list,
including the defaults you want to keep, and restart:

```
MCP_REDIRECT_URIS=https://claude.ai/api/mcp/auth_callback,https://claude.com/api/mcp/auth_callback,http://localhost:*,http://127.0.0.1:*,https://agent.example/oauth/callback
```

## What the agent can do

| Tool | What it does |
|---|---|
| `data_catalog`, `data_get` | read data items (values, tables with rows) |
| `data_set_value`, `data_create_table`, `data_update_table_columns` | create and change items |
| `data_rows_upsert`, `data_rows_delete` | change table rows |
| `data_set_public`, `data_delete` | make an item public or private, delete it |
| `asset_upload` | upload an image for image values and cells |
| `page_guide` | how a page is built here (also the resource `homelander://guide/page`) |
| `page_list_versions`, `page_get_version` | read page versions and their files |
| `page_create_draft`, `page_update_draft`, `page_copy_to_draft` | write drafts |

There is no tool to publish, unpublish or delete a page version.

## Technical notes

- OAuth 2.1 with PKCE (S256), Homelander as its own authorization server: metadata at
  `/.well-known/oauth-protected-resource/mcp` and `/.well-known/oauth-authorization-server`,
  dynamic client registration at `/oauth/register`, tokens at `/oauth/token`, revocation at
  `/oauth/revoke`. The consent page is `/oauth/authorize` in the web app.
- Access tokens last 1 hour, refresh tokens 30 days and are replaced on every use. Only hashes of
  codes and tokens are stored.
- At most 300 MCP requests a minute per connected agent; 30 requests a minute per address on the
  OAuth endpoints.
