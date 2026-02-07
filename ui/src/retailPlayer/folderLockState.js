import { normalizeValue } from './deviceUtils'

const UNLOCKED_STORAGE_KEY = 'retailPlayerUnlockedFolders'

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

const getFolderIdentifiers = (folder) => {
  const id = normalizeValue(folder?.id)
  return id ? [id] : []
}

export const isFolderLocked = (folder) => Boolean(folder?.isLocked)

export const markFolderUnlockedForSession = (folder) => {
  const identifiers = getFolderIdentifiers(folder)
  if (!identifiers.length) {
    return
  }

  const unlockedMap = readStorageMap(UNLOCKED_STORAGE_KEY)
  identifiers.forEach((identifier) => {
    unlockedMap[identifier] = true
  })
  writeStorageMap(UNLOCKED_STORAGE_KEY, unlockedMap)
}

export const isFolderUnlockedForSession = (folder) => {
  const identifiers = getFolderIdentifiers(folder)
  if (!identifiers.length) {
    return false
  }

  const unlockedMap = readStorageMap(UNLOCKED_STORAGE_KEY)
  return identifiers.some((identifier) => unlockedMap[identifier] === true)
}

export const getFolderLockIdentifiers = getFolderIdentifiers
