export const measuredNumber = (value) =>
  value !== null &&
  value !== undefined &&
  value !== '' &&
  Number.isFinite(Number(value))
    ? Number(value)
    : null

// Keep both readings from the same snapshot. Missing current peaks must not
// silently fall back to peaks measured from the original file.
// Restore clears the after snapshot; a later analysis may populate it again.
export const currentMeasurement = (audit) => {
  const after =
    audit?.lufsAfter != null ||
    audit?.tpAfter != null ||
    audit?.status === 'processed'
  return {
    lufs: measuredNumber(after ? audit?.lufsAfter : audit?.lufsBefore),
    peak: measuredNumber(after ? audit?.tpAfter : audit?.tpBefore),
    bitRate: after ? audit?.bitrateAfter : audit?.bitrateBefore,
  }
}
