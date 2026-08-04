// The loudness a song has right now, for the LUFS column.
//
// The audit is asked first and the song's own tags are the fallback. It used to
// be tags only, which went blank on every normalized song the moment the
// library was rescanned: normalization writes loudnorm_final_lufs to the tags
// column but never into the file, and a rescan rebuilds that column from the
// file. The audit lives in its own table precisely so a rescan cannot touch it.
//
// lufsAfter is the measurement of the file as it now stands; lufsBefore is that
// same measurement for a song that was only ever analysed, never rewritten. A
// tag is still read for songs this server has never looked at, which may carry
// one written by whatever produced them.
const auditLufs = (song) => {
  const audit = song?.loudnessAudit
  if (!audit) return null
  const value = audit.lufsAfter ?? audit.lufsBefore
  return value === null || value === undefined ? null : Number(value).toFixed(2)
}

const tagLufs = (song) => {
  const tags = song?.tags || {}
  // rawTags is only populated by the inspect endpoint, so it is usually absent
  // here - kept because it costs nothing and helps wherever it is present.
  const rawTags = song?.rawTags || {}

  const direct =
    tags.loudnorm_final_lufs?.[0] ??
    tags.final_lufs?.[0] ??
    tags.finallufs?.[0] ??
    tags.lufs?.[0]
  if (direct !== undefined && direct !== null && direct !== '') return direct

  const merged = { ...tags, ...rawTags }
  for (const [key, values] of Object.entries(merged)) {
    const normalized = key.toLowerCase()
    if (
      normalized.includes('loudnorm_final_lufs') ||
      normalized.includes('final_lufs') ||
      normalized.includes('finallufs')
    ) {
      return Array.isArray(values) ? (values[0] ?? '') : (values ?? '')
    }
  }

  return ''
}

export const getLufsValue = (song) => auditLufs(song) ?? tagLufs(song)
