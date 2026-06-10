import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { Script, createContext } from 'node:vm';

async function loadScratchCard() {
  const source = await readFile(new URL('./scratch-card.js', import.meta.url), 'utf8');
  const context = createContext({});
  new Script(source).runInContext(context);
  return context.scratchCard;
}

test('scratch card test marker requires standalone segment', async () => {
  const scratchCard = await loadScratchCard();

  assert.equal(scratchCard.isTestCardName('55-act-TEST'), true);
  assert.equal(scratchCard.isTestCardName('55_act test'), true);
  assert.equal(scratchCard.isTestCardName('55-act-contest'), false);
  assert.equal(scratchCard.isTestCardName('latest'), false);
});
