export const getLufsValue = (song) => {
  const tags = song?.tags || {}
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
