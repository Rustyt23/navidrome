import { deviceSlugKey, normalizeValue } from './deviceUtils'

const UNLOCKED_STORAGE_KEY = 'retailPlayerUnlockedDevices'

const readStorageMap = (key) => {
  if (typeof window === 'undefined') {
    return {}
  }

  try {
    const rawValue = window.sessionStorage.getItem(key)
    if (!rawValue) {
      return {}
    }
    const parsedValue = JSON.parse(rawValue)
    return parsedValue && typeof parsedValue === 'object' ? parsedValue : {}
  } catch (error) {
    return {}
  }
}

const writeStorageMap = (key, value) => {
  if (typeof window === 'undefined') {
    return
  }

  window.sessionStorage.setItem(key, JSON.stringify(value))
}

const getDeviceIdentifiers = (device) => {
  if (!device) {
    return []
  }

  const slug = normalizeValue(device?.slug || device?.name || device?.id)
  const identifiers = [
    slug ? deviceSlugKey(slug) : null,
    normalizeValue(device?.apiId),
    normalizeValue(device?.id),
  ].filter(Boolean)

  return identifiers.filter((value, index, array) => array.indexOf(value) === index)
}

export const isDeviceLocked = (device) => Boolean(device?.isLocked)

export const markDeviceUnlockedForSession = (device) => {
  const identifiers = getDeviceIdentifiers(device)
  if (!identifiers.length) {
    return
  }

  const unlockedMap = readStorageMap(UNLOCKED_STORAGE_KEY)
  identifiers.forEach((identifier) => {
    unlockedMap[identifier] = true
  })
  writeStorageMap(UNLOCKED_STORAGE_KEY, unlockedMap)
}

export const isDeviceUnlockedForSession = (device) => {
  const identifiers = getDeviceIdentifiers(device)
  if (!identifiers.length) {
    return false
  }

  const unlockedMap = readStorageMap(UNLOCKED_STORAGE_KEY)
  return identifiers.some((identifier) => unlockedMap[identifier] === true)
}

export const clearRetailPlayerLockSessionState = () => {
  if (typeof window === 'undefined') {
    return
  }

  window.sessionStorage.removeItem(UNLOCKED_STORAGE_KEY)
}

export const getDeviceLockIdentifiers = getDeviceIdentifiers
