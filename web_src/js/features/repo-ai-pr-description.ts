import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {ComboMarkdownEditor, getComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {triggerEditorContentChanged} from './comp/EditorMarkdown.ts';

type Editor = ComboMarkdownEditor;

function getEditorContainer(): HTMLElement | null {
  return document.querySelector<HTMLElement>('.combo-markdown-editor');
}

// The instance is attached to its container by the constructor, but init() is async and
// `textarea` is only assigned by setupTextarea(). value() dereferences that field, so it
// throws until init resolves.
function getEditor(): Editor | null {
  const container = getEditorContainer();
  if (!container) return null;
  const editor = getComboMarkdownEditor(container);
  if (!editor?.textarea) return null;
  return editor as Editor;
}

// The editor is initialised by a separate async init pass, so it can still be settling when
// the button is wired up. Give it a moment instead of failing the interaction.
async function waitForEditor(timeoutMs = 3000): Promise<Editor | null> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const editor = getEditor();
    if (editor) return editor;
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  return null;
}

function getButton(): HTMLButtonElement | null {
  return document.getElementById('ai-generate-description') as HTMLButtonElement | null;
}

type Labels = {
  generate: string;
  regenerate: string;
  generating: string;
  overwrite: string;
  failed: string;
};

function getLabels(): Labels {
  const btn = getButton();
  return {
    generate: btn?.dataset.labelGenerate || 'Generate Description',
    regenerate: btn?.dataset.labelRegenerate || 'Regenerate Description',
    generating: btn?.dataset.labelGenerating || 'Generating description…',
    overwrite: btn?.dataset.labelOverwrite || 'The description field already contains content. Overwrite?',
    failed: btn?.dataset.labelFailed || 'Unable to generate description: %s',
  };
}

function setButtonLabel(hasContent: boolean) {
  const btn = getButton();
  if (!btn || btn.disabled) return;
  btn.textContent = hasContent ? getLabels().regenerate : getLabels().generate;
}

function isRegenerate(editor: Editor): boolean {
  return editor.value().trim() !== '';
}

async function handleGenerateClick(e: Event) {
  e.preventDefault();
  const btn = e.currentTarget as HTMLButtonElement;
  if (btn.disabled) return;

  const editor = await waitForEditor();
  if (!editor) {
    showErrorToast('The description editor is not ready yet. Please try again.');
    return;
  }

  const labels = getLabels();
  if (isRegenerate(editor) && !window.confirm(labels.overwrite)) return;

  const base = btn.dataset.base;
  const head = btn.dataset.head;
  const url = btn.dataset.url;
  if (!base || !head || !url) {
    showErrorToast('Unable to determine base and head branches');
    return;
  }

  const statusEl = document.getElementById('ai-generate-description-status');
  const restoreLabel = isRegenerate(editor) ? labels.regenerate : labels.generate;

  btn.disabled = true;
  btn.textContent = labels.generating;
  if (statusEl) statusEl.textContent = labels.generating;

  try {
    const resp = await POST(url, {
      data: {base, head},
      headers: {'X-Gitea-Fetch-Action': '1'},
    });
    if (!resp.ok) {
      let message = `${resp.status} ${resp.statusText}`;
      try {
        const data = await resp.json();
        // Backend only ever returns a generic message, never provider details.
        message = data.message || data.errorMessage || message;
      } catch {}
      throw new Error(message);
    }
    const data = await resp.json();
    const description = data.description as string;
    if (!description) throw new Error('empty response');

    editor.value(description);
    triggerEditorContentChanged(editor.textarea);
    btn.textContent = labels.regenerate;
    if (statusEl) statusEl.textContent = '';
  } catch (err) {
    showErrorToast(labels.failed.replace('%s', (err as Error).message));
    btn.textContent = restoreLabel;
    if (statusEl) statusEl.textContent = '';
  } finally {
    btn.disabled = false;
  }
}

export function initRepoAIPullDescription() {
  const btn = getButton();
  if (!btn) return;

  // Only reflect existing content once the editor is actually usable.
  waitForEditor().then((editor) => {
    if (!editor) return;
    setButtonLabel(isRegenerate(editor));
    editor.container.addEventListener(ComboMarkdownEditor.EventEditorContentChanged, () => {
      if (editor.textarea) setButtonLabel(isRegenerate(editor));
    });
  });

  btn.addEventListener('click', handleGenerateClick);
}
