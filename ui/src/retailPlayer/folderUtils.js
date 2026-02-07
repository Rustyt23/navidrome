import { normalizeValue } from './deviceUtils'

const normalizeFolderId = (value) => {
  if (typeof value === 'string' && value.trim() !== '') {
    return value.trim()
  }
  return ''
}

const mapRetailPlayerFolder = (folder) => {
  if (!folder || typeof folder !== 'object') {
    return null
  }

  const id = normalizeFolderId(folder.id)
  if (!id) {
    return null
  }

  const name = normalizeValue(folder.name) || id
  const parentId = normalizeFolderId(folder.parentId) || null
  const isLocked =
    typeof folder.isLocked === 'boolean'
      ? folder.isLocked
      : typeof folder.is_locked === 'boolean'
        ? folder.is_locked
        : false

  return {
    id,
    name,
    parentId,
    isLocked,
    createdAt: folder.createdAt,
    updatedAt: folder.updatedAt,
  }
}

export { mapRetailPlayerFolder }
