const normalizeValue = (value) => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value)
  }
  if (typeof value === 'string') {
    return value.trim()
  }
  return ''
}

const buildDeviceSlug = (device) => {
  if (!device || typeof device !== 'object') {
    return ''
  }

  const name = normalizeValue(device.name)
  if (name) {
    return name
  }

  const id = normalizeValue(device.id)
  if (id) {
    return id
  }

  const macAddress = normalizeValue(device.macAddress)
  if (macAddress) {
    return macAddress
  }

  if (device.ordinal !== undefined && device.ordinal !== null) {
    const ordinal = Number(device.ordinal)
    if (!Number.isNaN(ordinal)) {
      return String(ordinal)
    }
  }

  return ''
}

const deviceSlugKey = (value) => normalizeValue(value).toLowerCase()

export { buildDeviceSlug, deviceSlugKey, normalizeValue }
