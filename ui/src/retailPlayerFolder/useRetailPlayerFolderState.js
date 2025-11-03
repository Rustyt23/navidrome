import { useCallback, useEffect, useMemo, useState } from 'react'
import { v4 as uuidv4 } from 'uuid'

const STORAGE_KEY = 'retailPlayerFolderState'

const normalizeParentId = (parentId) => (parentId === undefined ? null : parentId)

const loadInitialState = (devices) => {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored)
      return ensureStateShape(parsed, devices)
    }
  } catch (err) {
    // ignore storage parsing issues and fall back to defaults
  }

  return createInitialState(devices)
}

const createInitialState = (devices) => ({
  folders: [],
  assignments: devices.reduce((acc, device) => {
    acc[device.id] = null
    return acc
  }, {}),
})

const ensureStateShape = (state, devices) => {
  const safeState = {
    folders: Array.isArray(state?.folders) ? state.folders : [],
    assignments: typeof state?.assignments === 'object' && state.assignments
      ? { ...state.assignments }
      : {},
  }

  const knownDeviceIds = new Set(devices.map((device) => device.id))

  devices.forEach((device) => {
    if (!(device.id in safeState.assignments)) {
      safeState.assignments[device.id] = null
    }
  })

  Object.keys(safeState.assignments).forEach((deviceId) => {
    if (!knownDeviceIds.has(deviceId)) {
      delete safeState.assignments[deviceId]
    }
  })

  const folderIds = new Set(safeState.folders.map((folder) => folder.id))
  Object.entries(safeState.assignments).forEach(([deviceId, folderId]) => {
    if (folderId && !folderIds.has(folderId)) {
      safeState.assignments[deviceId] = null
    }
  })

  return safeState
}

const persistState = (state) => {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch (err) {
    // ignore storage errors
  }
}

const buildFolderMap = (folders) => {
  const map = new Map()
  folders.forEach((folder) => {
    map.set(folder.id, folder)
  })
  return map
}

const collectDescendantIds = (folderId, folders) => {
  const descendants = new Set()
  const toVisit = [folderId]

  while (toVisit.length > 0) {
    const current = toVisit.pop()
    descendants.add(current)

    folders.forEach((folder) => {
      const parentId = normalizeParentId(folder.parentId)
      if (parentId === current && !descendants.has(folder.id)) {
        toVisit.push(folder.id)
      }
    })
  }

  return descendants
}

const buildFolderOptions = (folders) => {
  const childrenMap = new Map()
  folders.forEach((folder) => {
    const parentId = normalizeParentId(folder.parentId)
    if (!childrenMap.has(parentId)) {
      childrenMap.set(parentId, [])
    }
    childrenMap.get(parentId).push(folder)
  })

  const walk = (parentId = null, depth = 0, acc = []) => {
    const siblings = childrenMap.get(parentId) || []
    const sorted = [...siblings].sort((a, b) => a.name.localeCompare(b.name))
    sorted.forEach((folder) => {
      acc.push({ ...folder, depth })
      walk(folder.id, depth + 1, acc)
    })
    return acc
  }

  return walk()
}

const useRetailPlayerFolderState = (devices) => {
  const [state, setState] = useState(() => loadInitialState(devices))

  useEffect(() => {
    setState((prev) => {
      const next = ensureStateShape(prev, devices)
      if (next !== prev) {
        persistState(next)
      }
      return next
    })
  }, [devices])

  const updateState = useCallback((updater) => {
    setState((prev) => {
      const next = typeof updater === 'function' ? updater(prev) : updater
      persistState(next)
      return next
    })
  }, [])

  const createFolder = useCallback((name, parentId = null) => {
    const now = new Date().toISOString()
    const ownerName = localStorage.getItem('username') || 'admin'
    updateState((prev) => ({
      folders: [
        ...prev.folders,
        {
          id: uuidv4(),
          name: name.trim(),
          parentId: normalizeParentId(parentId),
          ownerName,
          public: false,
          createdAt: now,
          updatedAt: now,
        },
      ],
      assignments: { ...prev.assignments },
    }))
  }, [updateState])

  const updateFolder = useCallback((folderId, updates) => {
    updateState((prev) => ({
      folders: prev.folders.map((folder) =>
        folder.id === folderId
          ? {
              ...folder,
              ...updates,
              updatedAt: updates.updatedAt || new Date().toISOString(),
            }
          : folder,
      ),
      assignments: { ...prev.assignments },
    }))
  }, [updateState])

  const toggleFolderPublic = useCallback((folderId) => {
    updateState((prev) => ({
      folders: prev.folders.map((folder) =>
        folder.id === folderId
          ? { ...folder, public: !folder.public, updatedAt: new Date().toISOString() }
          : folder,
      ),
      assignments: { ...prev.assignments },
    }))
  }, [updateState])

  const moveFolder = useCallback((folderId, targetParentId = null) => {
    updateState((prev) => {
      const normalizedTarget = normalizeParentId(targetParentId)
      const invalidTargets = collectDescendantIds(folderId, prev.folders)
      if (invalidTargets.has(normalizedTarget)) {
        return prev
      }
      return {
        folders: prev.folders.map((folder) =>
          folder.id === folderId
            ? { ...folder, parentId: normalizedTarget, updatedAt: new Date().toISOString() }
            : folder,
        ),
        assignments: { ...prev.assignments },
      }
    })
  }, [updateState])

  const moveDevice = useCallback((deviceId, targetParentId = null) => {
    updateState((prev) => ({
      folders: prev.folders,
      assignments: {
        ...prev.assignments,
        [deviceId]: normalizeParentId(targetParentId),
      },
    }))
  }, [updateState])

  const deleteFolder = useCallback((folderId) => {
    updateState((prev) => {
      const descendants = collectDescendantIds(folderId, prev.folders)
      const folderMap = buildFolderMap(prev.folders)
      const folder = folderMap.get(folderId)
      const parentId = normalizeParentId(folder?.parentId)

      const remainingFolders = prev.folders.filter((item) => !descendants.has(item.id))
      const assignments = { ...prev.assignments }
      Object.entries(assignments).forEach(([deviceId, assignedFolder]) => {
        if (assignedFolder && descendants.has(assignedFolder)) {
          assignments[deviceId] = parentId
        }
      })

      return {
        folders: remainingFolders,
        assignments,
      }
    })
  }, [updateState])

  const foldersMap = useMemo(() => buildFolderMap(state.folders), [state.folders])

  const getBreadcrumb = useCallback((folderId) => {
    if (!folderId) {
      return []
    }
    const path = []
    let current = foldersMap.get(folderId)
    const safetyCounter = state.folders.length + 1
    let guard = 0
    while (current && guard < safetyCounter) {
      path.unshift(current)
      const parentId = normalizeParentId(current.parentId)
      current = parentId ? foldersMap.get(parentId) : null
      guard += 1
    }
    return path
  }, [foldersMap, state.folders.length])

  const folderOptions = useMemo(() => buildFolderOptions(state.folders), [state.folders])

  const getItems = useCallback(
    (parentId = null, searchTerm = '', sortField = 'name', sortOrder = 'asc') => {
      const normalizedParent = normalizeParentId(parentId)
      const folders = state.folders
        .filter((folder) => normalizeParentId(folder.parentId) === normalizedParent)
        .map((folder) => ({
          id: folder.id,
          type: 'folder',
          name: folder.name,
          ownerName: folder.ownerName,
          updatedAt: folder.updatedAt,
          public: folder.public,
          data: folder,
        }))

      const devicesInFolder = devices
        .filter((device) => normalizeParentId(state.assignments[device.id]) === normalizedParent)
        .map((device) => ({
          id: device.id,
          type: 'device',
          name: device.name,
          ownerName: device.organization || '',
          updatedAt: null,
          public: null,
          data: device,
        }))

      const merged = [...folders, ...devicesInFolder]
      const term = searchTerm.trim().toLowerCase()
      const filtered = term
        ? merged.filter((item) =>
            item.name.toLowerCase().includes(term) ||
            (item.ownerName || '').toLowerCase().includes(term),
          )
        : merged

      const direction = sortOrder.toLowerCase() === 'desc' ? -1 : 1

      filtered.sort((a, b) => {
        const resolveValue = (item) => {
          switch (sortField) {
            case 'ownerName':
              return (item.ownerName || '').toLowerCase()
            case 'updatedAt':
              return item.updatedAt || ''
            case 'public':
              return item.type === 'folder' ? (item.public ? 1 : 0) : -1
            case 'type':
              return item.type
            default:
              return item.name.toLowerCase()
          }
        }

        const valueA = resolveValue(a)
        const valueB = resolveValue(b)

        if (valueA < valueB) return -1 * direction
        if (valueA > valueB) return 1 * direction
        return 0
      })

      return filtered
    },
    [devices, state.assignments, state.folders],
  )

  const getFolderById = useCallback((folderId) => foldersMap.get(folderId) || null, [foldersMap])

  const getDescendantIds = useCallback(
    (folderId) => {
      if (!folderId) {
        return new Set()
      }
      return collectDescendantIds(folderId, state.folders)
    },
    [state.folders],
  )

  return {
    folders: state.folders,
    assignments: state.assignments,
    createFolder,
    updateFolder,
    toggleFolderPublic,
    deleteFolder,
    moveFolder,
    moveDevice,
    getBreadcrumb,
    getItems,
    folderOptions,
    getFolderById,
    getDescendantIds,
  }
}

export default useRetailPlayerFolderState
