// ── fzt DOM Renderer ───────────────────────────────────────────
// Renders fzt state as native DOM elements instead of parsing ANSI.
// Used with the structured data API (getVisibleRows, getPromptState, getUIState).
//
// Usage:
//   import { renderDOM } from './fzt-dom-renderer.js';
//   renderDOM(container, rows, prompt, ui);

/**
 * Render the full fzt UI from structured data.
 * @param {HTMLElement} container - The container element
 * @param {Array} rows - From fzt.getVisibleRows()
 * @param {Object} prompt - From fzt.getPromptState()
 * @param {Object} ui - From fzt.getUIState()
 */
export function renderDOM(container, rows, prompt, ui) {
  container.innerHTML = '';
  container.classList.add('fzt-dom');

  if (ui.border) {
    const titleBar = createTitleBar(ui);
    container.appendChild(titleBar);
  }

  const promptBar = createPromptBar(prompt);
  container.appendChild(promptBar);

  const treeContainer = document.createElement('div');
  treeContainer.className = 'fzt-tree';
  for (const row of rows) {
    treeContainer.appendChild(createTreeRow(row));
  }
  container.appendChild(treeContainer);
}

function createTitleBar(ui) {
  const bar = document.createElement('div');
  bar.className = 'fzt-title-bar';

  if (ui.label) {
    const label = document.createElement('span');
    label.className = 'fzt-label';
    label.textContent = ui.label;
    bar.appendChild(label);
  }

  if (ui.title) {
    const title = document.createElement('span');
    title.className = 'fzt-title';
    title.textContent = ui.title;
    bar.appendChild(title);
  }

  if (ui.version) {
    const ver = document.createElement('span');
    ver.className = 'fzt-version';
    ver.textContent = ui.version;
    bar.appendChild(ver);
  }

  return bar;
}

function createPromptBar(prompt) {
  const bar = document.createElement('div');
  bar.className = 'fzt-prompt';

  // Mode icon
  const icon = document.createElement('span');
  icon.className = prompt.mode === 'nav' ? 'fzt-prompt-icon fzt-nav' : 'fzt-prompt-icon fzt-search';
  icon.textContent = prompt.mode === 'nav' ? '\uF0A9' : '\uF002';
  bar.appendChild(icon);

  // Scope breadcrumbs
  for (const seg of (prompt.scopePath || [])) {
    const crumb = document.createElement('span');
    crumb.className = 'fzt-scope-crumb';
    crumb.textContent = seg + ' ';
    bar.appendChild(crumb);
  }

  // Query + ghost
  if (prompt.query) {
    const q = document.createElement('span');
    q.className = 'fzt-query';
    q.textContent = prompt.query;
    bar.appendChild(q);

    if (prompt.ghost) {
      const ghost = document.createElement('span');
      ghost.className = 'fzt-ghost';
      ghost.textContent = prompt.ghost;
      bar.appendChild(ghost);
    }
  } else if (prompt.hint) {
    const hint = document.createElement('span');
    hint.className = 'fzt-hint';
    hint.textContent = prompt.hint;
    bar.appendChild(hint);
  }

  // Cursor
  const cursor = document.createElement('span');
  cursor.className = 'fzt-cursor';
  bar.appendChild(cursor);

  return bar;
}

function createTreeRow(row) {
  const div = document.createElement('div');
  div.className = 'fzt-row';
  if (row.isSelected) div.classList.add('fzt-selected');
  if (row.isTopMatch) div.classList.add('fzt-top-match');

  // Selection indicator
  const indicator = document.createElement('span');
  indicator.className = 'fzt-indicator';
  indicator.textContent = row.isSelected ? '\u25B8 ' : '  ';
  div.appendChild(indicator);

  // Indent
  if (row.depth > 0) {
    const indent = document.createElement('span');
    indent.className = 'fzt-indent';
    indent.style.width = (row.depth * 2) + 'ch';
    indent.style.display = 'inline-block';
    div.appendChild(indent);
  }

  // Icon
  const icon = document.createElement('span');
  icon.className = row.isFolder ? 'fzt-icon fzt-folder-icon' : 'fzt-icon fzt-file-icon';
  icon.textContent = row.isFolder ? '\U000F024B' : '\uF15B';
  div.appendChild(icon);

  // Name with match highlighting
  const name = document.createElement('span');
  name.className = row.isFolder ? 'fzt-name fzt-folder-name' : 'fzt-name';
  if (row.nameMatchIndices && row.nameMatchIndices.length > 0) {
    name.appendChild(highlightText(row.name, row.nameMatchIndices));
  } else {
    name.textContent = row.name;
  }
  div.appendChild(name);

  // Description
  if (row.description) {
    const desc = document.createElement('span');
    desc.className = 'fzt-desc';
    if (row.descMatchIndices && row.descMatchIndices.length > 0) {
      desc.appendChild(highlightText(row.description, row.descMatchIndices));
    } else {
      desc.textContent = row.description;
    }
    div.appendChild(desc);
  }

  return div;
}

function highlightText(text, indices) {
  const frag = document.createDocumentFragment();
  const indexSet = new Set(indices);
  const chars = [...text];
  let i = 0;

  while (i < chars.length) {
    if (indexSet.has(i)) {
      // Collect consecutive highlighted chars
      let end = i;
      while (end < chars.length && indexSet.has(end)) end++;
      const span = document.createElement('span');
      span.className = 'fzt-match';
      span.textContent = chars.slice(i, end).join('');
      frag.appendChild(span);
      i = end;
    } else {
      // Collect consecutive normal chars
      let end = i;
      while (end < chars.length && !indexSet.has(end)) end++;
      frag.appendChild(document.createTextNode(chars.slice(i, end).join('')));
      i = end;
    }
  }

  return frag;
}
