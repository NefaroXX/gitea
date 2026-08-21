# AI Pull Request Description Generation

Gitea can generate Pull Request descriptions using a configurable OpenAI-compatible LLM endpoint.

## Configuration

Add an `[ai]` section to your `app.ini`:

```ini
[ai]
ENABLED = true
BASE_URL = https://api.openai.com/v1
API_KEY = sk-...
MODEL = gpt-3.5-turbo
```

For local/self-hosted providers (Ollama, vLLM, LocalAI):

```ini
[ai]
ENABLED = true
BASE_URL = http://localhost:11434/v1
API_KEY =
MODEL = llama3
```

### OpenAI-compatible endpoints

`BASE_URL` must be the base URL without the trailing `/chat/completions`. Examples:

- OpenAI: `https://api.openai.com/v1`
- OpenRouter: `https://openrouter.ai/api/v1`
- Ollama: `http://localhost:11434/v1`
- vLLM: `http://localhost:8000/v1`
- LocalAI: `http://localhost:8080/v1`

`API_KEY` is sent as `Authorization: Bearer <key>`. For local providers that don't require authentication, leave it empty.

### Limits

```ini
[ai]
MAX_DIFF_BYTES = 50000
MAX_COMMITS = 20
MAX_FILES = 50
MAX_PROMPT_BYTES = 100000
REQUEST_TIMEOUT = 30
```

- `MAX_DIFF_BYTES`: maximum diff bytes sent to the model. Large diffs are truncated; the model is told the diff was truncated.
- `MAX_COMMITS`: maximum commits included.
- `MAX_FILES`: maximum changed files included.
- `MAX_PROMPT_BYTES`: maximum total prompt size. If exceeded, the diff is truncated further.
- `REQUEST_TIMEOUT`: HTTP timeout in seconds.

## How it works

```
PR creation page -> Generate Description button -> Gitea backend
  -> determine base/head, merge-base, commits, diff/statistics, PR template
  -> construct focused prompt -> POST to LLM provider
  -> Markdown description -> inserted into existing PR description editor
```

The user can freely edit the generated description before creating the PR.

## Security / Privacy

- Repository contents (diffs, commit messages, file lists) are sent to the configured LLM provider. **Do not enable this feature if your repositories contain sensitive data and your provider is external.**
- The browser never calls the LLM provider directly. The server is responsible for the request.
- `API_KEY` and provider credentials are never exposed to the frontend and are never logged.
- The endpoint `POST /api/v1/repos/{owner}/{repo}/pulls/generate-description` (and the web endpoint `POST /{owner}/{repo}/compare/generate-description`) checks that the authenticated user has `write` permission on pull requests and respects repository visibility.
- Provider requests have a timeout and are bounded by `REQUEST_TIMEOUT`.

## Usage

1. Enable `[ai]` in `app.ini` and restart Gitea.
2. Open the **New Pull Request** page (`/compare/...`).
3. Click **Generate Description** near the description editor.
4. While generating, the button shows loading state and is disabled to prevent duplicate requests.
5. On success, Markdown is inserted into the existing editor; you can edit it.
6. If the editor already contains text, you will be asked to confirm before overwriting. After generation the button becomes **Regenerate Description**.

If generation fails, an error toast is shown but PR creation remains usable. If AI is disabled or unavailable, normal PR creation still works.

## API

### `POST /api/v1/repos/{owner}/{repo}/pulls/generate-description`

Request:

```json
{
  "base": "main",
  "head": "feature/example"
}
```

Response:

```json
{
  "description": "## Summary\n\n..."
}
```

Authentication: token or session with `write` permission on pull requests. Requires `write:repository` scope for tokens.

Error codes:

- `403` – no permission
- `503` – AI disabled or provider unavailable
- `504` – provider timeout
- `400` – invalid base/head or no common history
