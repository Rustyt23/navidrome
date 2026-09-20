// Mirrors model.LoudnessComparisonEpsilon: absorb binary rounding at the
// inclusive LUFS boundary without widening the measured loudness range.
export const withinLoudnessTolerance = (lufs, target, tolerance) =>
  Number.isFinite(lufs) &&
  Number.isFinite(target) &&
  Number.isFinite(tolerance) &&
  Math.abs(lufs - target) <= tolerance + 1e-9

// Mirrors model.LoudnessTruePeakToleranceDB. A true peak is reconstructed
// rather than read, and encoding shifts it unpredictably, so a finished file is
// allowed this much above the configured ceiling. At the -0.5 default that puts
// the worst shipped peak at -0.4, still well clear of where clipping starts.
export const TRUE_PEAK_TOLERANCE_DB = 0.1

// The highest true peak a finished file may carry - model.LoudnessShippingCeiling.
//
// Every verdict this page prints has to use it. Judged against the bare ceiling
// the page called a file unsafe that the engine had accepted on purpose, which
// put finished songs on the decision list and dropped them from the on-target
// count. The engine, the SQL and these pages now read one bound.
export const shippingCeiling = (ceiling) => ceiling + TRUE_PEAK_TOLERANCE_DB

// peakAcceptable answers "may this measured peak ship?". An unmeasured peak
// never can: nothing is known about it.
export const peakAcceptable = (peak, ceiling) =>
  peak !== null && Number.isFinite(peak) && peak <= shippingCeiling(ceiling)
