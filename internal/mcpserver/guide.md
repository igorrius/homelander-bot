# How to build the Homelander site

Homelander shows every page of the site as **our header + your page in a sandboxed frame**: the
home page at `/`, other pages at their own addresses (`/contacts`, `/rules`, …). You write the site
as a set of files (a *version*); an admin previews the draft and publishes the whole site at once in
the admin panel. You cannot publish: after `page_create_draft` or `page_update_draft`, give the admin the
`admin_url` you got back and ask them to look.

## 1. Files

- `index.html` is required; it is the home page `/`. Every other `*.html` in the root is a page too
  (see "Several pages").
- Paths are relative, like `index.html`, `style.css`, `js/app.js`, `img/logo.png`: letters, digits,
  `.`, `_` and `-`, folders separated by `/`; extensions in lower case. The top folder `preview/`
  is reserved.
- Allowed types: `.html`, `.css`, `.js`, `.json`, `.svg`, `.png` (APNG too), `.jpg`/`.jpeg`, `.webp`,
  `.gif`, `.woff2`.
- Limits: 5 MiB per file, 20 MiB and 200 files per version.
- Send text files as `text`, binary files as `base64` (standard base64) — exactly one per file.
- Refer to your files with relative URLs (`style.css`, `img/logo.png`). The published page is served
  from `/site/`, a draft from `/site/preview/<token>/`, so absolute paths like `/style.css` break.

## 2. The sandbox

Your page is served with this Content-Security-Policy (it replaces any `<meta>` policy you add):

```
sandbox allow-scripts allow-popups allow-popups-to-escape-sandbox allow-top-navigation-by-user-activation;
default-src 'self' data: blob:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'
```

What that means:

- The page has an opaque origin: **no cookies, no localStorage of the site, no session**. It cannot
  call the admin API or see who is logged in. Our header above the frame does login and language.
- **No external resources**: no CDNs, web fonts, analytics or images from other sites. Upload any
  library or font as a file of the version. Inline `<script>` and `<style>` are fine.
- `fetch` may reach only this site (`connect-src 'self'`) — that is how you read public data.
- Links that should open in the whole window (for example to `/login` or `/register`) need
  `target="_top"`: `<a href="/register" target="_top">Join</a>`. Other links may use
  `target="_blank"`.

## 3. Reading data

The page reads **public** data items at view time, so when an admin changes a value the page shows
it without a new version. Use `data_catalog` to see which keys exist and which are public;
`data_set_public` makes an item public. Private items are never returned.

- `GET /api/v1/public/data` — every public item in one object keyed by item key:

```json
{
  "house.address": { "kind": "value", "type": "text", "value": "1 Example Street" },
  "house.name": { "kind": "value", "type": "localized_text", "value": { "uk": "Будинок", "en": "House" } },
  "house.photo": { "kind": "value", "type": "image", "value": "/api/v1/assets/12" },
  "contacts": {
    "kind": "table",
    "columns": [
      { "name": "name", "label": "Name", "type": "text" },
      { "name": "phone", "label": "Phone", "type": "phone" }
    ],
    "rows": [ { "id": 1, "name": "Office", "phone": "+380000000000" } ]
  }
}
```

- `GET /api/v1/public/data/<key>` — one item in the same shape; `404` when it is missing or private.
- Value types: `text`, `localized_text` (`{ "uk": ..., "en": ... }`, either may be missing),
  `number`, `bool`, `image` (a URL like `/api/v1/assets/<id>`), `url` (an `http(s)` link), `phone`
  (digits, spaces, `+ ( ) -`; link it as `tel:` with the spaces removed), `json`. Table column types:
  `text`, `localized_text`, `number`, `bool`, `image` (URL), `url`, `phone`. An empty cell is missing or `null`.
- Responses are cached for 30 seconds and rate-limited per visitor (about 120 requests a minute):
  load data once when the page opens, not in a loop.
- The keys above are examples. Never put real addresses or phone numbers into page files: keep them
  in the data space and read them at view time.

```js
async function loadData() {
  const res = await fetch('/api/v1/public/data')
  if (!res.ok) return {}
  return res.json()
}
```

## 4. Height: tell the header how tall the page is

The frame does not grow by itself. Post your height to the parent on load and whenever it changes:

```html
<script>
  function sendHeight() {
    const height = document.documentElement.scrollHeight
    parent.postMessage({ type: 'homelander:height', height }, '*')
  }
  new ResizeObserver(sendHeight).observe(document.documentElement)
  window.addEventListener('load', sendHeight)
</script>
```

Heights between 100 and 20000 pixels are accepted. Avoid `height: 100vh` on the body: the frame
height comes from your content.

## 5. Language: Ukrainian and English

The frame is opened with `?lang=uk` or `?lang=en`; when the visitor switches language in the
header, the page receives a `homelander:lang` message instead of a reload:

```html
<script>
  let lang = new URLSearchParams(location.search).get('lang') === 'en' ? 'en' : 'uk'
  function pick(text) {
    // localized_text values look like { uk: '...', en: '...' }
    if (text && typeof text === 'object') return text[lang] || text.uk || text.en || ''
    return text ?? ''
  }
  window.addEventListener('message', (e) => {
    if (e.source === parent && e.data && e.data.type === 'homelander:lang') {
      lang = e.data.lang === 'en' ? 'en' : 'uk'
      render() // your function that redraws the texts
    }
  })
</script>
```

Ukrainian is the default; write both languages for your own texts.

## 6. Several pages

- Every `*.html` file in the root of a version is a page: `index.html` → `/`, `contacts.html` →
  `/contacts`, `rules.html` → `/rules`. The whole site is published together, so a link between
  pages never points to a page that is not live.
- Page names: lower-case Latin letters, digits and `-` (`^[a-z0-9][a-z0-9-]{0,62}$`). These names are
  **reserved** (they are app addresses) and are refused with `files: reserved_page_name`: `admin`,
  `login`, `register`, `oauth`, `site`, `api`, `mcp`, `healthz`, `assets`, `telegram`, `favicon`,
  `robots`, `sitemap`. Any other bad name is refused with `files: invalid_page_name`. An `.html` file
  in a folder (`parts/menu.html`) is an ordinary file, not a page.
- Link pages by their address and **always with `target="_top"`**, so the browser address and the
  Back button follow: `<a href="/contacts" target="_top">Contacts</a>`, `<a href="/" target="_top">Home</a>`.
  Write links the same way for drafts: in the admin's preview they switch the previewed page
  (Homelander injects a small `homelander:navigate` script into previews only — you do not send or
  handle that message yourself).
- Share CSS and JS between pages as files (`style.css`, `js/app.js`) and include them in every page
  with relative URLs. Each page sends its own height (section 4) and handles `homelander:lang`
  (section 5); the language the visitor chose carries over between pages.
- Every tool result lists the version's `pages` with their addresses: check it after each change.

## 7. Working with drafts

- `page_list_versions` lists versions (draft, published, archived); `page_get_version` returns a
  version's files with the content of text files.
- `page_create_draft` makes a new draft; `page_update_draft` puts or deletes files of a draft;
  published and archived versions cannot change — use `page_copy_to_draft` to start from one.
- Validation errors name the field and the file, e.g. `validation failed: files: type (path=x.ico)`
  or `files: reserved_page_name (path=admin.html)`.
- To change a live site: `page_list_versions` → `page_get_version` of the published one →
  `page_copy_to_draft` → `page_update_draft` with only the files you change → give the admin the
  `admin_url`.
- The admin opens `admin_url`, previews the draft in the real header and frame, and publishes it.
  Tell them what you changed and what to check.
