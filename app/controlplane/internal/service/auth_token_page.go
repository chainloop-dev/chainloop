//
// Copyright 2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"fmt"
	"html/template"
	"net/http"
)

var tokenPageTemplate = template.Must(template.New("tokenPage").Parse(tokenPageHTML))

// renderTokenPage serves a self-contained HTML page that displays the JWT
// to the user with a copy-to-clipboard button. The token is rendered in the
// response body (never in the URL), and headers prevent caching or referrer
// leakage so the bearer token does not escape the page.
func renderTokenPage(w http.ResponseWriter, token string) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")

	if err := tokenPageTemplate.Execute(w, struct{ Token string }{Token: token}); err != nil {
		return fmt.Errorf("failed to render token page: %w", err)
	}
	return nil
}

// #nosec G101 -- HTML template, not a credential
const tokenPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Chainloop — Your authentication token</title>
<style>
  :root {
    color-scheme: light dark;
    --bg: #f6f7f9;
    --card: #ffffff;
    --fg: #1f2330;
    --muted: #5a6478;
    --border: #e2e5ec;
    --accent: #6366f1;
    --accent-hover: #4f52d6;
    --code-bg: #0f172a;
    --code-fg: #e5e7eb;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #0b0d12;
      --card: #151821;
      --fg: #e5e7eb;
      --muted: #9aa3b2;
      --border: #262a36;
      --code-bg: #0a0d14;
      --code-fg: #e5e7eb;
    }
  }
  * { box-sizing: border-box; }
  html, body { margin: 0; padding: 0; height: 100%; }
  body {
    background: var(--bg);
    color: var(--fg);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
  }
  .card {
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 28px;
    max-width: 640px;
    width: 100%;
    box-shadow: 0 10px 30px rgba(0,0,0,0.06);
  }
  h1 { font-size: 20px; margin: 0 0 8px; }
  p { margin: 0 0 16px; color: var(--muted); line-height: 1.5; }
  .token {
    display: block;
    background: var(--code-bg);
    color: var(--code-fg);
    border-radius: 8px;
    padding: 14px 16px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
    font-size: 13px;
    line-height: 1.4;
    word-break: break-all;
    overflow-wrap: anywhere;
    white-space: pre-wrap;
    max-height: 240px;
    overflow-y: auto;
  }
  .actions { margin-top: 16px; display: flex; gap: 8px; align-items: center; }
  button {
    appearance: none;
    border: 0;
    background: var(--accent);
    color: #fff;
    padding: 10px 14px;
    font-size: 14px;
    font-weight: 600;
    border-radius: 8px;
    cursor: pointer;
    transition: background-color 120ms ease;
  }
  button:hover { background: var(--accent-hover); }
  button:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
  .status { font-size: 13px; color: var(--muted); }
  .status.ok { color: #10b981; }
</style>
</head>
<body>
  <main class="card">
    <h1>You're authenticated</h1>
    <p>Copy the token below and paste it into your terminal to complete the login.</p>
    <code class="token" id="token">{{.Token}}</code>
    <div class="actions">
      <button type="button" id="copy-btn" onclick="copyToken()">Copy Token</button>
      <span class="status" id="status" aria-live="polite"></span>
    </div>
  </main>
  <script>
    async function copyToken() {
      const token = document.getElementById('token').textContent;
      const status = document.getElementById('status');
      try {
        await navigator.clipboard.writeText(token);
        status.textContent = 'Copied to clipboard';
        status.classList.add('ok');
        setTimeout(() => { status.textContent = ''; status.classList.remove('ok'); }, 2000);
      } catch (e) {
        status.textContent = 'Copy failed — select the token and copy manually';
      }
    }
  </script>
</body>
</html>`
