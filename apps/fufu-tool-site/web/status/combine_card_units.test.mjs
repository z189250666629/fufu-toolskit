import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function loadCardUnits() {
  const context = { window: {} };
  context.globalThis = context.window;
  vm.runInNewContext(
    readFileSync(new URL('./combine_card_units.js', import.meta.url), 'utf8'),
    context
  );
  return context.window.combineCardUnits;
}

test('getAllowedUnitsForUser normalizes string interval units', () => {
  const { getAllowedUnitsForUser } = loadCardUnits();

  assert.deepEqual(Array.from(getAllowedUnitsForUser([{ interval_unit: '8' }])), [8]);
  assert.deepEqual(Array.from(getAllowedUnitsForUser([{ interval_unit: '3' }, { interval_unit: 8 }])), [8]);
  assert.deepEqual(Array.from(getAllowedUnitsForUser([{ interval_unit: '09' }, { interval_unit: '3' }])), [8, 9]);
});
