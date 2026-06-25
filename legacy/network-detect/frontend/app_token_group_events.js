import { selectTokenGroupState } from './app_state.js';
import {
  isTokenGroupOptionKey,
  isTokenGroupTriggerOpenKey,
  nextTokenGroupOptionIndex
} from './app_event_keys.js';

export function bindTokenGroupSelectEvents({
  documentRef = document,
  windowRef = window,
  state,
  renderWithMotion
}) {
  const tokenGroupTrigger = documentRef.querySelector('[data-token-group-trigger]');
  const requestFrame = windowRef.requestAnimationFrame?.bind(windowRef) || ((callback) => callback());

  tokenGroupTrigger?.addEventListener('click', () => {
    state.groupSelectOpen = !state.groupSelectOpen;
    renderWithMotion('select');
  });

  tokenGroupTrigger?.addEventListener('keydown', (event) => {
    if (!isTokenGroupTriggerOpenKey(event.key)) return;
    event.preventDefault();
    state.groupSelectOpen = true;
    renderWithMotion('select');
    requestFrame(() => {
      const selected = documentRef.querySelector('[data-token-group-option][aria-selected="true"]');
      (selected || documentRef.querySelector('[data-token-group-option]'))?.focus();
    });
  });

  documentRef.querySelectorAll('[data-token-group-option]').forEach((button) => {
    button.addEventListener('click', () => {
      selectTokenGroupState(state, button.dataset.tokenGroupOption || '');
      renderWithMotion('scope');
    });

    button.addEventListener('keydown', (event) => {
      if (!isTokenGroupOptionKey(event.key)) return;
      event.preventDefault();
      const options = [...documentRef.querySelectorAll('[data-token-group-option]')];
      if (event.key === 'Escape') {
        state.groupSelectOpen = false;
        renderWithMotion('select');
        requestFrame(() => documentRef.querySelector('[data-token-group-trigger]')?.focus());
        return;
      }
      const nextIndex = nextTokenGroupOptionIndex(event.key, options.indexOf(button), options.length);
      if (nextIndex != null) options[nextIndex]?.focus();
    });
  });
}
