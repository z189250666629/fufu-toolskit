import test from 'node:test';
import assert from 'node:assert/strict';
import { batchRanges, parsePositiveIntOption } from './upload-core.mjs';

test('parsePositiveIntOption distinguishes an absent option from a missing value', () => {
  assert.equal(parsePositiveIntOption([], 'batch-size'), null);
  assert.throws(
    () => parsePositiveIntOption(['--batch-size'], 'batch-size'),
    /--batch-size 必须是正整数/
  );
});

test('parsePositiveIntOption accepts safe positive integers only', () => {
  assert.equal(parsePositiveIntOption(['--batch-size', '500'], 'batch-size'), 500);
  for (const value of ['0', '-1', '1.5', 'abc', '9007199254740992']) {
    assert.throws(
      () => parsePositiveIntOption(['--batch-size', value], 'batch-size'),
      /--batch-size 必须是正整数/
    );
  }
});

test('batchRanges yields one range at a time for explicit batching', () => {
  assert.deepEqual([...batchRanges(5, 2)], [[0, 2], [2, 4], [4, 5]]);
});

test('batchRanges keeps the default upload as one range', () => {
  assert.deepEqual([...batchRanges(5, null)], [[0, 5]]);
  assert.deepEqual([...batchRanges(0, null)], []);
});
