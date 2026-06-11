import {
  escapeHtml
} from './utils.js';

export function renderSegmentedTabs({
  className,
  ariaLabel,
  motionKey,
  activeValue,
  options = [],
  buttonClassName,
  getControls,
  getId,
  dataAttribute
}) {
  const activeIndex = Math.max(0, options.findIndex((option) => option.value === activeValue));
  const resolvedActiveValue = options[activeIndex]?.value;
  const tabCount = Math.max(1, options.length);

  return `
    <div
      class="${escapeHtml(className)}"
      role="tablist"
      aria-label="${escapeHtml(ariaLabel)}"
      data-slot="tab-list"
      data-tab-motion-key="${escapeHtml(motionKey)}"
      data-orientation="horizontal"
      style="--tab-count: ${tabCount}; --active-tab-index: ${activeIndex};"
    >
      <span class="tab-indicator tabs__indicator" data-slot="tab-indicator" aria-hidden="true"></span>
      ${options.map((option) => {
        const active = resolvedActiveValue === option.value;
        return `
        <button
          class="${escapeHtml(buttonClassName)} ${active ? 'active' : ''}"
          type="button"
          role="tab"
          aria-selected="${active ? 'true' : 'false'}"
          aria-controls="${escapeHtml(getControls(option.value))}"
          id="${escapeHtml(getId(option.value))}"
          tabindex="${active ? '0' : '-1'}"
          data-slot="tab"
          data-selected="${active ? 'true' : 'false'}"
          data-${dataAttribute}="${escapeHtml(option.value)}"
        >
          <span class="tab-label" data-slot="tab-label">${escapeHtml(option.label)}</span>
        </button>
      `;
      }).join('')}
    </div>
  `;
}
