import {POST} from '../modules/fetch.ts';
import {showErrorToast, showSuccessToast} from '../modules/toast.ts';
import {getComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

const {appSubUrl} = window.config;

function getEditor(): any {
  const container = document.querySelector('.combo-markdown-editor') as HTMLElement;
  if (!container) return null;
  return getComboMarkdownEditor(container);
}

function parseBaseHeadFromUrl(): {base: string, head: string} | null {
  // URL pattern: /{owner}/{repo}/compare/{base}...{head}  or  /compare/{base}..{head}
  const path = window.location.pathname;
  const idx = path.indexOf('/compare/');
  if (idx === -1) return null;
  let comparePart = path.slice(idx + '/compare/'.length);
  // comparePart may contain "..." or ".."
  let sep = '...';
  let parts = comparePart.split('...');
  if (parts.length !== 2) {
    sep = '..';
    parts = comparePart.split('..');
    if (parts.length !== 2) return null;
  }
  // parts[0] is base (may include owner/repo: prefix when fork), parts[1] is head
  // For simplicity, extract last segment after last '/' or ':'
  const extractBranch = (s: string) => {
    // If s contains '/', possibly like "owner/repo:branch" or "branch", take after last ':' then last '/'
    const afterColon = s.includes(':') ? s.slice(s.lastIndexOf(':') + 1) : s;
    // After colon, there may be still '/', but branch may contain slashes, so we need full branch name after first '/compare/' base part handling is tricky
    // Instead, we will directly use the split parts as base and head raw, then extract via decode
    // For base: may be "main" or "user/repo:main"? But compare API expects base param separate from head param handling in backend via ParseCompareRouterParam which handles owner/repo prefix.
    // For AI we just need branch names, so we can send baseBranch/headBranch as parsed from page data if available.
    return decodeURIComponent(afterColon);
  };
  // Better: try to get from pageData or data attributes on button
  const btn = document.getElementById('ai-generate-description') as HTMLElement | null;
  if (btn) {
    const b = btn.getAttribute('data-base');
    const h = btn.getAttribute('data-head');
    if (b && h) return {base: b, head: h};
  }
  // fallback to URL parsing: assume base and head are simple branch names without slash
  return {base: extractBranch(parts[0]), head: extractBranch(parts[1])};
}

function getRepoOwnerName(): {owner: string, repo: string} | null {
  // Try to parse from window.location or from repo link data
  const btn = document.getElementById('ai-generate-description') as HTMLElement | null;
  const link = btn?.getAttribute('data-repo-link'); // e.g., /user/repo
  if (link) {
    const parts = link.replace(/^\//, '').split('/');
    if (parts.length >= 2) return {owner: parts[0], repo: parts[1]};
  }
  // fallback to URL: /{owner}/{repo}/compare/...
  const m = window.location.pathname.match(/^\/([^/]+)\/([^/]+)\/compare\//);
  if (m) return {owner: m[1], repo: m[2]};
  return null;
}

async function handleGenerateClick(e: Event) {
  e.preventDefault();
  const btn = e.currentTarget as HTMLButtonElement;
  if (btn.disabled) return;

  const editor = getEditor();
  if (!editor) {
    showErrorToast('Editor not found');
    return;
  }

  const currentContent = editor.value().trim();
  const isRegenerate = currentContent !== '';
  const btnOriginalText = btn.textContent;
  const statusEl = document.getElementById('ai-generate-description-status');

  if (isRegenerate) {
    const confirmed = window.confirm('The description field already contains content. Overwrite it?');
    if (!confirmed) return;
  }

  const baseHead = parseBaseHeadFromUrl();
  if (!baseHead) {
    showErrorToast('Unable to determine base and head branches');
    return;
  }

  // Prefer data-url from button if present
  const btnUrl = btn.getAttribute('data-url');
  let url = btnUrl || '';
  if (!url) {
    const repoInfo = getRepoOwnerName();
    if (repoInfo) {
      // Prefer web endpoint: /{owner}/{repo}/compare/generate-description
      url = `${appSubUrl}/${repoInfo.owner}/${repoInfo.repo}/compare/generate-description`;
    } else {
      // fallback to API endpoint
      const ownerRepo = window.location.pathname.split('/').slice(1, 3).join('/');
      url = `${appSubUrl}/api/v1/repos/${ownerRepo}/pulls/generate-description`;
    }
  }

  btn.disabled = true;
  btn.classList.add('is-loading');
  const prevText = btn.textContent;
  btn.textContent = 'Generating description...';
  if (statusEl) statusEl.textContent = 'Generating description...';

  try {
    const resp = await POST(url, {
      data: {base: baseHead.base, head: baseHead.head},
      headers: {'X-Gitea-Fetch-Action': '1'},
    });
    if (!resp.ok) {
      const text = await resp.text();
      let msg = `Unable to generate description: ${resp.statusText}`;
      try {
        const json = JSON.parse(text);
        if (json.message) msg = `Unable to generate description: ${json.message}`;
        else if (json.errorMessage) msg = `Unable to generate description: ${json.errorMessage}`;
      } catch {}
      // Handle specific codes
      if (resp.status === 503) msg = 'AI is disabled or provider unavailable';
      if (resp.status === 403) msg = 'You do not have permission to generate description';
      throw new Error(msg);
    }
    const data = await resp.json();
    const description = data.description || data.message || '';
    if (!description) {
      throw new Error('Empty response from AI provider');
    }
    editor.value(description);
    // trigger content changed event so other handlers update
    const container = document.querySelector('.combo-markdown-editor') as HTMLElement;
    if (container) {
      container.dispatchEvent(new CustomEvent('ce-editor-content-changed'));
      container.dispatchEvent(new Event('input', {bubbles: true}));
    }
    // Update button text to Regenerate
    btn.textContent = 'Regenerate Description';
    if (statusEl) {
      statusEl.textContent = '';
    }
    showSuccessToast('Description generated');
  } catch (err: any) {
    const msg = err?.message || 'Unable to generate description';
    showErrorToast(msg);
    if (statusEl) statusEl.textContent = msg;
    if (btnOriginalText) btn.textContent = btnOriginalText;
    else btn.textContent = prevText;
  } finally {
    btn.disabled = false;
    btn.classList.remove('is-loading');
    if (!btn.textContent || btn.textContent === 'Generating description...') {
      btn.textContent = currentContent ? 'Regenerate Description' : 'Generate Description';
    }
    if (statusEl && statusEl.textContent === 'Generating description...') {
      statusEl.textContent = '';
    }
  }
}

function initAIGenerateDescription() {
  const btn = document.getElementById('ai-generate-description') as HTMLButtonElement | null;
  if (!btn) return;
  // Determine initial text based on editor content
  const editor = getEditor();
  if (editor) {
    const content = editor.value().trim();
    if (content) btn.textContent = 'Regenerate Description';
    // Listen for editor changes to toggle text?
    const container = document.querySelector('.combo-markdown-editor') as HTMLElement;
    if (container) {
      container.addEventListener('ce-editor-content-changed', () => {
        const c = editor.value().trim();
        if (!btn.disabled) {
          btn.textContent = c ? 'Regenerate Description' : 'Generate Description';
        }
      });
    }
  }
  btn.addEventListener('click', handleGenerateClick);
}

registerGlobalInitFunc('initAIGenerateDescription', initAIGenerateDescription);

// For direct import
export function initRepoAIPullDescription() {
  initAIGenerateDescription();
}
