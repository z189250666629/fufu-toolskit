import { createDashboardApp } from './app_controller.js';

createDashboardApp({
  documentRef: document,
  windowRef: window,
  navigatorRef: navigator,
  intlRef: Intl
}).boot();
