(function(root) {
  function isTestCardName(name) {
    return String(name ?? '')
      .trim()
      .split(/[-_\s.]+/)
      .some((part) => part.toLowerCase() === 'test');
  }

  root.scratchCard = {
    isTestCardName
  };
})(globalThis);
