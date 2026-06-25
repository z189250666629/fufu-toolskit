(function registerCombineCardUnits(global) {
  function normalizeIntervalUnit(value) {
    const number = Number(value);
    return Number.isFinite(number) ? number : 0;
  }

  function uniqueSortedUnits(units) {
    return Array.from(new Set(units.filter((unit) => unit > 0))).sort((a, b) => a - b);
  }

  function getAllowedUnitsForUser(tokens) {
    const items = Array.isArray(tokens) ? tokens : [];
    if (!items.length) return [3, 8, 9];

    const units = uniqueSortedUnits(items.map((token) => normalizeIntervalUnit(token?.interval_unit)));
    if (items.length === 1) {
      return units;
    }

    const allowed = new Set();
    if (units.includes(3)) allowed.add(8);
    if (units.includes(8)) allowed.add(8);
    if (units.includes(9)) allowed.add(9);
    return Array.from(allowed);
  }

  global.combineCardUnits = {
    getAllowedUnitsForUser,
    normalizeIntervalUnit
  };
})(typeof window !== 'undefined' ? window : globalThis);
