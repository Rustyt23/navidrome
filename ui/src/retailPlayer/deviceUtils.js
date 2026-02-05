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


const normalizeLockedValue = (value, fallback = false) => {
  if (typeof value === 'boolean') {
    return value
  }

  if (typeof value === 'number' && Number.isFinite(value)) {
    if (value === 1) {
      return true
    }
    if (value === 0) {
      return false
    }
  }

  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (!normalized) {
      return fallback
    }
    if (['true', '1', 'yes', 'y', 'on', 'locked'].includes(normalized)) {
      return true
    }
    if (['false', '0', 'no', 'n', 'off', 'unlocked'].includes(normalized)) {
      return false
    }
  }

  return fallback
}

const resolveLockedField = (device) => {
  if (!device || typeof device !== 'object') {
    return undefined
  }

  if (Object.prototype.hasOwnProperty.call(device, 'locked')) {
    return device.locked
  }
  if (Object.prototype.hasOwnProperty.call(device, 'isLocked')) {
    return device.isLocked
  }
  if (Object.prototype.hasOwnProperty.call(device, 'is_locked')) {
    return device.is_locked
  }
  if (Object.prototype.hasOwnProperty.call(device, 'lock')) {
    return device.lock
  }

  return undefined
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
  const remoteControlId = normalizeValue(device.remoteControlId)
  const locked = normalizeLockedValue(resolveLockedField(device))

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
    remoteControlId,
    locked,
  }
}

export {
  buildDeviceSlug,
  deviceSlugKey,
  mapRetailPlayerDevice,
  normalizeLockedValue,
  normalizeValue,
  resolveLockedField,
}
