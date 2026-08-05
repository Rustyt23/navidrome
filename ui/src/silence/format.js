// Formatting helpers for the silence page.
//
// The page is about small amounts of time - a second here, five there - so the
// usual mm:ss duration format is the wrong unit: "0:03" reads as a length,
// while "3.2s" reads as an amount removed, which is what these numbers are.

// formatSeconds renders a trim amount. Two decimals below ten seconds, because
// the difference between 0.52s and 0.61s is the difference between a margin
// that was kept and one that was not, and one decimal above it, where that
// precision stops meaning anything.
export const formatSeconds = (value) => {
  if (value === null || value === undefined) return '-'
  const seconds = Number(value)
  if (!Number.isFinite(seconds)) return '-'
  if (seconds === 0) return '0s'
  if (Math.abs(seconds) < 10) return `${seconds.toFixed(2)}s`
  if (Math.abs(seconds) < 60) return `${seconds.toFixed(1)}s`
  const mins = Math.floor(Math.abs(seconds) / 60)
  const rest = Math.abs(seconds) % 60
  const sign = seconds < 0 ? '-' : ''
  return `${sign}${mins}m ${rest.toFixed(0)}s`
}

// formatTotalTime renders a library-wide total, where minutes and hours are the
// units that mean something.
export const formatTotalTime = (value) => {
  const seconds = Number(value) || 0
  if (seconds < 60) return `${seconds.toFixed(1)} seconds`
  if (seconds < 3600) {
    const mins = Math.floor(seconds / 60)
    const rest = Math.round(seconds % 60)
    return `${mins} min ${rest} sec`
  }
  const hours = Math.floor(seconds / 3600)
  const mins = Math.round((seconds % 3600) / 60)
  return `${hours} h ${mins} min`
}

export const formatBytes = (value) => {
  const bytes = Number(value) || 0
  if (bytes === 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB']
  let n = Math.abs(bytes)
  let unit = 0
  while (n >= 1024 && unit < units.length - 1) {
    n /= 1024
    unit += 1
  }
  return `${bytes < 0 ? '-' : ''}${n.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`
}

// Why a song with silence was left alone, in words rather than in the stored
// slug. Each one says what was found, not just that something was.
export const SKIP_REASON_LABELS = {
  audible: 'Something audible in it',
  fade: 'Measurement unreliable',
  too_long: 'Too much to be dead air',
  gapless: 'Album plays continuously',
  unsupported: 'Format cannot be cut safely',
  too_short: 'Less than the margin',
}

// The level at or below which audio is treated as nothing. Mirrors
// ffmpeg.SilentPeakDB - the page states the threshold it is judging against, so
// the number in the Evidence column can be read rather than taken on trust.
export const SILENT_PEAK_DB = -50

// formatPeakDb renders a measured peak. Digital silence has no real peak, and
// ffmpeg reports the format's floor for it, so anything that low is shown as
// silence rather than as a number nobody can interpret.
export const formatPeakDb = (value) => {
  if (value === null || value === undefined) return '-'
  const db = Number(value)
  if (!Number.isFinite(db) || db <= -90) return 'silent'
  return `${db.toFixed(1)} dB`
}

export const skipReasonLabel = (reason) => SKIP_REASON_LABELS[reason] || reason

export const VERDICT_LABELS = {
  clean: 'Nothing to remove',
  trimmable: 'Ready to trim',
  skipped: 'Left alone',
  failed: 'Could not measure',
}

export const METHOD_LABELS = {
  copy: 'Lossless (bit-exact)',
  encode: 'Re-encoded',
}
