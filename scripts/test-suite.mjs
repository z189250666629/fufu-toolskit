import { NODE_TEST_FILES } from './test-config.mjs';

for (const testFile of NODE_TEST_FILES) {
  await import(new URL(`../${testFile}`, import.meta.url));
}
