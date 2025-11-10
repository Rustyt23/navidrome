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

const deviceSlugKey = (value) => {
  const normalized = normalizeValue(value).toLowerCase()
  if (!normalized) {
    return ''
  }
  return normalized.replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

const mapRetailPlayerDevice = (device) => {
  if (!device || typeof device !== 'object') {
    return null
  }

  const rawId = normalizeValue(device.id)
  const fallbackId =
    rawId ||
    normalizeValue(device.macAddress || device.mac_address) ||
    normalizeValue(device.ordinal)

  if (!fallbackId) {
    return null
  }

  const slug = buildDeviceSlug(device) || fallbackId
  const name = normalizeValue(device.name) || fallbackId
  const folderIds = Array.isArray(device.folderIds)
    ? device.folderIds
        .map((value) => (typeof value === 'string' ? value.trim() : ''))
        .filter(Boolean)
    : []

  return {
    id: fallbackId,
    apiId: rawId || null,
    name,
    slug,
    slugKey: deviceSlugKey(slug),
    channel: normalizeValue(device.channel),
    channelList: normalizeValue(device.channelList),
    organization:
      normalizeValue(device.organization) ||
      normalizeValue(device.orgUnit || device.org_unit) ||
      normalizeValue(device.location),
    timeZone: normalizeValue(device.timeZone || device.time_zone),
    folderIds,
  }
}

export { buildDeviceSlug, deviceSlugKey, mapRetailPlayerDevice, normalizeValue }
