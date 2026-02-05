import { baseUrl } from '../utils'

const LOCKED_DEVICES_STORAGE_KEY = 'retailPlayerLockedDevices'
const UNLOCKED_DEVICES_STORAGE_KEY = 'retailPlayerUnlockedDevices'

const normalizeIdentifier = (value) => {
  if (typeof value !== 'string') {
    return ''
  }

  return value.trim().toLowerCase()
}

const parseStorageMap = (storageKey) => {
  if (typeof window === 'undefined') {
    return {}
  }

  try {
    const rawValue = window.sessionStorage.getItem(storageKey)
    if (!rawValue) {
      return {}
    }

    const parsed = JSON.parse(rawValue)
    if (!parsed || typeof parsed !== 'object') {
      return {}
    }

    return Object.keys(parsed).reduce((accumulator, key) => {
      const normalizedKey = normalizeIdentifier(key)
      if (normalizedKey && parsed[key] === true) {
        accumulator[normalizedKey] = true
      }
      return accumulator
    }, {})
  } catch (error) {
    return {}
  }
}

const writeStorageMap = (storageKey, map) => {
  if (typeof window === 'undefined') {
    return
  }

  window.sessionStorage.setItem(storageKey, JSON.stringify(map))
}

const getDeviceIdentifiers = (device) => {
  if (!device) {
    return []
  }

  const sourceValues =
    typeof device === 'string'
      ? [device]
      : [
          device.apiId,
          device.id,
          device.slug,
          device.name,
          device.macAddress,
          device.mac_address,
        ]

  return sourceValues
    .map(normalizeIdentifier)
    .filter(
      (value, index, values) =>
        Boolean(value) && values.indexOf(value) === index,
    )
}

const readLockedMap = () => parseStorageMap(LOCKED_DEVICES_STORAGE_KEY)
const readUnlockedMap = () => parseStorageMap(UNLOCKED_DEVICES_STORAGE_KEY)

export const isDeviceLocked = (device) => {
  const identifiers = getDeviceIdentifiers(device)
  if (!identifiers.length) {
    return false
  }

  const lockedMap = readLockedMap()
  return identifiers.some((identifier) => lockedMap[identifier] === true)
}

export const isDeviceUnlockedForSession = (device) => {
  const identifiers = getDeviceIdentifiers(device)
  if (!identifiers.length) {
    return false
  }

  const unlockedMap = readUnlockedMap()
  return identifiers.some((identifier) => unlockedMap[identifier] === true)
}

export const setDeviceLocked = (device, locked) => {
  const identifiers = getDeviceIdentifiers(device)
  if (!identifiers.length) {
    return
  }

  const lockedMap = readLockedMap()
  const unlockedMap = readUnlockedMap()

  identifiers.forEach((identifier) => {
    if (locked) {
      lockedMap[identifier] = true
      delete unlockedMap[identifier]
    } else {
      delete lockedMap[identifier]
      delete unlockedMap[identifier]
    }
  })

  writeStorageMap(LOCKED_DEVICES_STORAGE_KEY, lockedMap)
  writeStorageMap(UNLOCKED_DEVICES_STORAGE_KEY, unlockedMap)
}

export const setDeviceUnlockedForSession = (device, unlocked) => {
  const identifiers = getDeviceIdentifiers(device)
  if (!identifiers.length) {
    return
  }

  const unlockedMap = readUnlockedMap()
  identifiers.forEach((identifier) => {
    if (unlocked) {
      unlockedMap[identifier] = true
    } else {
      delete unlockedMap[identifier]
    }
  })

  writeStorageMap(UNLOCKED_DEVICES_STORAGE_KEY, unlockedMap)
}

export const clearDeviceLockSession = () => {
  if (typeof window === 'undefined') {
    return
  }

  window.sessionStorage.removeItem(UNLOCKED_DEVICES_STORAGE_KEY)
}

export const verifyRetailPlayerLockPassword = async (password) => {
  const response = await fetch(
    baseUrl('/api/retailplayer/device-lock/verify'),
    {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ password }),
    },
  )

  if (response.status === 204) {
    return
  }

  if (response.status === 401) {
    throw new Error('INVALID_PASSWORD')
  }

  throw new Error('LOCK_SERVICE_ERROR')
}
