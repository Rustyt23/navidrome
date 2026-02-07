const LOCKED_FOLDERS_STORAGE_KEY = 'retailPlayerLockedFolders'
const UNLOCKED_FOLDERS_STORAGE_KEY = 'retailPlayerUnlockedFolders'

const readStorageMap = (storage, key) => {
  if (typeof window === 'undefined' || !storage) {
    return {}
  }

  try {
    const rawValue = storage.getItem(key)
    if (!rawValue) {
      return {}
    }
    const parsedValue = JSON.parse(rawValue)
    return parsedValue && typeof parsedValue === 'object' ? parsedValue : {}
  } catch (error) {
    return {}
  }
}

const writeStorageMap = (storage, key, value) => {
  if (typeof window === 'undefined' || !storage) {
    return
  }

  storage.setItem(key, JSON.stringify(value))
}

const getFolderIdentifiers = (folder) => {
  if (!folder) {
    return []
  }

  const identifiers = [folder.id].filter(Boolean)
  return identifiers.filter((value, index, array) => array.indexOf(value) === index)
}

export const getLockedFolderMap = () => {
  if (typeof window === 'undefined') {
    return {}
  }
  return readStorageMap(window.localStorage, LOCKED_FOLDERS_STORAGE_KEY)
}

export const isFolderLocked = (folder, lockedMap) => {
  if (!folder?.id) {
    return false
  }
  const map = lockedMap || getLockedFolderMap()
  return Boolean(map[folder.id])
}

export const setFolderLockState = (folder, isLocked) => {
  if (!folder?.id) {
    return
  }

  const lockedMap = getLockedFolderMap()
  if (isLocked) {
    lockedMap[folder.id] = true
  } else {
    delete lockedMap[folder.id]
  }
  writeStorageMap(window.localStorage, LOCKED_FOLDERS_STORAGE_KEY, lockedMap)
}

export const markFolderUnlockedForSession = (folder) => {
  const identifiers = getFolderIdentifiers(folder)
  if (!identifiers.length) {
    return
  }

  const unlockedMap = readStorageMap(window.sessionStorage, UNLOCKED_FOLDERS_STORAGE_KEY)
  identifiers.forEach((identifier) => {
    unlockedMap[identifier] = true
  })
  writeStorageMap(window.sessionStorage, UNLOCKED_FOLDERS_STORAGE_KEY, unlockedMap)
}

export const isFolderUnlockedForSession = (folder) => {
  const identifiers = getFolderIdentifiers(folder)
  if (!identifiers.length) {
    return false
  }

  const unlockedMap = readStorageMap(window.sessionStorage, UNLOCKED_FOLDERS_STORAGE_KEY)
  return identifiers.some((identifier) => unlockedMap[identifier] === true)
}

export const clearFolderUnlockSession = (folder) => {
  if (typeof window === 'undefined') {
    return
  }

  if (!folder?.id) {
    return
  }

  const unlockedMap = readStorageMap(window.sessionStorage, UNLOCKED_FOLDERS_STORAGE_KEY)
  if (unlockedMap[folder.id]) {
    delete unlockedMap[folder.id]
    writeStorageMap(window.sessionStorage, UNLOCKED_FOLDERS_STORAGE_KEY, unlockedMap)
  }
}
