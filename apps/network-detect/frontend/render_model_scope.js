import {
  escapeHtml,
  modelScopeTabId,
  modelSiteDisplayName
} from './utils.js';

export function renderTokenGroupSelect(groups, selectedGroup, stateLike = {}) {
  const isOpen = stateLike.groupSelectOpen;
  const current = selectedGroup || groups[0] || '';

  return `
    <div class="field group-select heroui-select" data-slot="select" data-token-group-select>
      <span id="tokenGroupSelectLabel" data-slot="select-label">分组</span>
      <button
        class="heroui-select-trigger"
        id="tokenGroupSelect"
        type="button"
        data-slot="select-trigger"
        aria-haspopup="listbox"
        aria-expanded="${isOpen ? 'true' : 'false'}"
        ${isOpen ? 'aria-controls="tokenGroupListbox"' : ''}
        aria-labelledby="tokenGroupSelectLabel tokenGroupSelectValue"
        data-token-group-trigger
      >
        <span class="heroui-select-value" id="tokenGroupSelectValue" data-slot="select-value">${escapeHtml(current || '选择分组')}</span>
        <span class="heroui-select-indicator" data-slot="select-indicator" aria-hidden="true">
          <svg viewBox="0 0 20 20" focusable="false">
            <path d="M5.5 7.5 10 12l4.5-4.5" />
          </svg>
        </span>
      </button>
      ${isOpen ? `
        <div class="heroui-select-popover" data-slot="select-popover">
          <div class="heroui-select-listbox" id="tokenGroupListbox" role="listbox" data-slot="listbox" aria-labelledby="tokenGroupSelectLabel">
            ${groups.map((group) => `
              <button
                class="heroui-select-item"
                type="button"
                role="option"
                data-slot="listbox-item"
                data-selected="${group === current ? 'true' : 'false'}"
                aria-selected="${group === current ? 'true' : 'false'}"
                data-token-group-option="${escapeHtml(group)}"
              >
                <span>${escapeHtml(group)}</span>
                <span class="heroui-select-item-indicator" data-slot="select-item-indicator" aria-hidden="true">
                  ${group === current ? `
                    <svg viewBox="0 0 20 20" focusable="false">
                      <path d="m4.75 10.25 3.25 3.25 7.25-7.25" />
                    </svg>
                  ` : ''}
                </span>
              </button>
            `).join('')}
          </div>
        </div>
      ` : ''}
    </div>
  `;
}

export function renderModelScopeControls(modelStatus, scope, stateLike) {
  const sites = modelStatus.sites || [];
  const activeIndex = Math.max(0, sites.findIndex((item) => item.site.name === scope.siteName));
  const hasTokenGroups = scope.siteName === 'token-fufu';
  return `
    <div class="model-scope-bar">
      <div
        class="model-scope-tabs tabs__list"
        role="tablist"
        aria-label="模型站点"
        data-slot="tab-list"
        data-tab-motion-key="model-site"
        data-orientation="horizontal"
        style="--tab-count: ${Math.max(1, sites.length)}; --active-tab-index: ${activeIndex};"
      >
        <span class="tab-indicator tabs__indicator" data-slot="tab-indicator" aria-hidden="true"></span>
        ${sites.map((item) => {
          const active = item.site.name === scope.siteName;
          return `
          <button
            class="scope-button tabs__tab ${active ? 'active' : ''}"
            type="button"
            role="tab"
            aria-selected="${active ? 'true' : 'false'}"
            aria-controls="modelScopePanel"
            id="${modelScopeTabId(item.site.name)}"
            tabindex="${active ? '0' : '-1'}"
            data-slot="tab"
            data-selected="${active ? 'true' : 'false'}"
            data-model-site="${escapeHtml(item.site.name)}"
          >
            <span class="tab-label" data-slot="tab-label">${escapeHtml(modelSiteDisplayName(item.site.name))}</span>
          </button>
        `;
        }).join('')}
      </div>
      <div class="model-scope-group-slot${hasTokenGroups ? '' : ' is-placeholder'}" ${hasTokenGroups ? '' : 'aria-hidden="true"'}>
        ${hasTokenGroups ? renderTokenGroupSelect(scope.groups || [], scope.group, stateLike) : ''}
      </div>
    </div>
  `;
}
