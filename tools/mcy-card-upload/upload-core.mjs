export function parsePositiveIntOption(args, name) {
  const option = `--${name}`;
  const idx = args.indexOf(option);
  if (idx < 0) return null;

  const raw = args[idx + 1];
  if (raw == null || !/^[1-9]\d*$/.test(raw)) {
    throw new Error(`${option} 必须是正整数`);
  }

  const value = Number.parseInt(raw, 10);
  if (!Number.isSafeInteger(value)) {
    throw new Error(`${option} 必须是正整数`);
  }
  return value;
}

export function* batchRanges(length, batchSize) {
  if (length <= 0) return;
  if (batchSize == null) {
    yield [0, length];
    return;
  }

  for (let start = 0; start < length; start += batchSize) {
    yield [start, Math.min(start + batchSize, length)];
  }
}
