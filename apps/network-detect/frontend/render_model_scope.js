import {
  escapeHtml,
  modelScopeTabId,
  modelSiteDisplayName
} from './utils.js';
import {
  renderSegmentedTabs
} from './render_tabs.js';

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
  const siteOptions = sites.map((item) => ({
    value: item.site.name,
    label: modelSiteDisplayName(item.site.name)
  }));
  const hasTokenGroups = scope.siteName === 'token-fufu';
  return `
    <div class="model-scope-bar">
      ${renderSegmentedTabs({
        className: 'model-scope-tabs tabs__list',
        ariaLabel: '模型站点',
        motionKey: 'model-site',
        activeValue: scope.siteName,
        options: siteOptions,
        buttonClassName: 'scope-button tabs__tab',
        getControls: () => 'modelScopePanel',
        getId: modelScopeTabId,
        dataAttribute: 'model-site'
      })}
      <div class="model-scope-group-slot${hasTokenGroups ? '' : ' is-placeholder'}" ${hasTokenGroups ? '' : 'aria-hidden="true"'}>
        ${hasTokenGroups ? renderTokenGroupSelect(scope.groups || [], scope.group, stateLike) : ''}
      </div>
    </div>
  `;
}
