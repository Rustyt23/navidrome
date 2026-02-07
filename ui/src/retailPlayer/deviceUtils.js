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

const resolveChannelName = (device) => {
  if (!device || typeof device !== 'object') {
    return ''
  }

  const direct = normalizeValue(device.channelName)
  if (direct) {
    return direct
  }

  const snakeCase = normalizeValue(device.channel_name)
  if (snakeCase) {
    return snakeCase
  }

  const channelObject =
    device.channel && typeof device.channel === 'object'
      ? device.channel
      : null
  if (channelObject) {
    return normalizeValue(
      channelObject.name || channelObject.label || channelObject.title,
    )
  }

  return ''
}

const resolveChannelId = (device) => {
  if (!device || typeof device !== 'object') {
    return ''
  }

  if (
    typeof device.channel === 'string' ||
    typeof device.channel === 'number'
  ) {
    return normalizeValue(device.channel)
  }

  const channelObject =
    device.channel && typeof device.channel === 'object'
      ? device.channel
      : null
  if (channelObject) {
    return normalizeValue(channelObject.id || channelObject.channelId)
  }

  return ''
}

const mapRetailPlayerDevice = (device) => {
  if (!device || typeof device !== 'object') {
    return null
  }

  const rawId = normalizeValue(device.id)
  const fallbackId =
    rawId ||
    normalizeValue(device.macAddress) ||
    normalizeValue(device.ordinal)

  if (!fallbackId) {
    return null
  }

  const slug = buildDeviceSlug(device) || fallbackId
  const macAddress = normalizeValue(device.macAddress)
  const channelName = resolveChannelName(device)
  const name = normalizeValue(device.name) || fallbackId
  const folderIds = Array.isArray(device.folderIds)
    ? device.folderIds
        .map((value) => (typeof value === 'string' ? value.trim() : ''))
        .filter(Boolean)
    : []
  const remoteControlId = normalizeValue(device.remoteControlId)
  const isLocked =
    typeof device.isLocked === 'boolean'
      ? device.isLocked
      : typeof device.is_locked === 'boolean'
        ? device.is_locked
        : undefined
  const isOnline =
    typeof device.online === 'boolean'
      ? device.online
      : typeof device.isOnline === 'boolean'
        ? device.isOnline
        : undefined

  return {
    id: fallbackId,
    apiId: rawId || null,
    name,
    slug,
    slugKey: deviceSlugKey(slug),
    channel: resolveChannelId(device),
    channelName: channelName || resolveChannelId(device),
    channelList: normalizeValue(device.channelList),
    macAddress,
    organization:
      normalizeValue(device.organization) ||
      normalizeValue(device.orgUnit || device.org_unit) ||
      normalizeValue(device.location),
    timeZone: normalizeValue(device.timeZone || device.time_zone),
    folderIds,
    remoteControlId,
    ...(typeof isLocked === 'boolean' ? { isLocked } : {}),
    ...(typeof isOnline === 'boolean' ? { online: isOnline } : {}),
  }
}

export {
  buildDeviceSlug,
  deviceSlugKey,
  mapRetailPlayerDevice,
  normalizeValue,
  resolveChannelName,
}
