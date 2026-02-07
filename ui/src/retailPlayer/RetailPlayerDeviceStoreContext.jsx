import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
} from 'react'
import PropTypes from 'prop-types'
import { v4 as uuidv4 } from 'uuid'
import useRetailPlayerDevices from './useRetailPlayerDevices'
import httpClient from '../dataProvider/httpClient'
import { buildDeviceSlug, deviceSlugKey, normalizeValue } from './deviceUtils'

const RetailPlayerDeviceStoreContext = createContext(null)

const initialState = {
  folders: [],
  devices: [],
  loading: true,
  error: null,
  isApiEnabled: false,
  lastUpdated: null,
}

const ensureFolderId = (value) => {
  if (typeof value === 'string' && value.trim() !== '') {
    return value
  }
  return null
}

const normalizeFolderIds = (value) => {
  if (Array.isArray(value)) {
    return Array.from(new Set(value.map(ensureFolderId).filter(Boolean)))
  }
  const single = ensureFolderId(value)
  return single ? [single] : []
}

const normalizeTimestamp = (value, fallback) => {
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = new Date(value)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed.toISOString()
    }
  }
  if (fallback instanceof Date) {
    return fallback.toISOString()
  }
  if (typeof fallback === 'string' && fallback.trim() !== '') {
    const parsed = new Date(fallback)
    if (!Number.isNaN(parsed.getTime())) {
      return parsed.toISOString()
    }
  }
  return new Date().toISOString()
}

const normalizeFolderRecord = (folder, existing) => {
  if (!folder || typeof folder !== 'object') {
    return existing || null
  }

  const existingFolder = existing || null
  const id = ensureFolderId(folder.id) || existingFolder?.id || uuidv4()
  const name = normalizeValue(folder.name) || existingFolder?.name || 'New Folder'

  let parentId = existingFolder?.parentId || null
  if (Object.prototype.hasOwnProperty.call(folder, 'parentId')) {
    const normalizedParent = ensureFolderId(folder.parentId)
    parentId = normalizedParent && normalizedParent !== id ? normalizedParent : null
  }

  const createdAt = normalizeTimestamp(
    folder.createdAt,
    existingFolder?.createdAt || new Date(),
  )
  const updatedAt = normalizeTimestamp(
    folder.updatedAt,
    folder.createdAt ? folder.createdAt : existingFolder?.updatedAt || createdAt,
  )

  return {
    id,
    name,
    parentId,
    createdAt,
    updatedAt,
  }
}

const baseDeviceShape = (device, existing) => {
  const normalizedName = normalizeValue(device?.name)
  const normalizedSlug = normalizeValue(device?.slug)
  const normalizedChannel = normalizeValue(device?.channel)
  const normalizedChannelList = normalizeValue(device?.channelList)
  const normalizedChannelName = normalizeValue(device?.channelName)
  const normalizedChannelCatalogCount = Number.isFinite(device?.channelCatalogCount)
    ? device.channelCatalogCount
    : Number.isFinite(existing?.channelCatalogCount)
      ? existing.channelCatalogCount
      : null
  const normalizedMacAddress = normalizeValue(device?.macAddress)
  const normalizedOrganization = normalizeValue(device?.organization)
  const normalizedTimeZone = normalizeValue(device?.timeZone)
  const normalizedRemoteControlId = normalizeValue(device?.remoteControlId)
  const normalizedIsLocked =
    typeof device?.isLocked === 'boolean'
      ? device.isLocked
      : typeof existing?.isLocked === 'boolean'
        ? existing.isLocked
        : false
  const normalizedIsOnline =
    typeof device?.online === 'boolean'
      ? device.online
      : typeof existing?.online === 'boolean'
        ? existing.online
        : undefined

  const existingFolderIds = normalizeFolderIds(
    existing?.folderIds ?? existing?.folderId,
  )
  const incomingFolderIds = normalizeFolderIds(
    device?.folderIds ?? device?.folderId ?? device?.folders,
  )
  const folderIds = incomingFolderIds.length ? incomingFolderIds : existingFolderIds
  const primaryFolderId = folderIds.length ? folderIds[0] : null

  const slug =
    normalizedSlug ||
    (existing?.slug || buildDeviceSlug(device) || normalizedName || device?.id)

  return {
    id: existing?.id || device?.id || uuidv4(),
    apiId: normalizeValue(device?.apiId || device?.id) || null,
    name: normalizedName || existing?.name || slug || 'Device',
    slug,
    slugKey: deviceSlugKey(slug),
    channel: normalizedChannel || '',
    channelList: normalizedChannelList || '',
    channelName: normalizedChannelName || normalizedChannel || '',
    channelCatalogCount: normalizedChannelCatalogCount,
    macAddress: normalizedMacAddress || existing?.macAddress || '',
    organization: normalizedOrganization || '',
    timeZone: normalizedTimeZone || '',
    remoteControlId: normalizedRemoteControlId || '',
    isLocked: normalizedIsLocked,
    ...(typeof normalizedIsOnline === 'boolean' ? { online: normalizedIsOnline } : {}),
    folderIds,
    folderId: primaryFolderId,
    source: existing?.source === 'local' ? 'local' : 'remote',
    attributes: existing?.attributes || {},
  }
}

const reducer = (state, action) => {
  switch (action.type) {
    case 'SET_LOADING':
      return { ...state, loading: Boolean(action.payload) }
    case 'SET_ERROR':
      return { ...state, error: action.payload || null }
    case 'SET_API_ENABLED':
      return { ...state, isApiEnabled: Boolean(action.payload) }
    case 'SYNC_REMOTE': {
      const incomingDevices = Array.isArray(action.payload?.devices)
        ? action.payload.devices
        : []
      const incomingFolders = Array.isArray(action.payload?.folders)
        ? action.payload.folders
        : []
      const incomingAssignments = Array.isArray(action.payload?.deviceFolders)
        ? action.payload.deviceFolders
        : []

      const normalizedFolders = incomingFolders
        .map((folder) => normalizeFolderRecord(folder))
        .filter(Boolean)

      const folderIdSet = new Set(normalizedFolders.map((folder) => folder.id))
      const assignmentMap = new Map()
      incomingAssignments.forEach((assignment) => {
        if (!assignment || typeof assignment !== 'object') {
          return
        }
        const deviceId = normalizeValue(assignment.deviceId || assignment.deviceID)
        const folderId = ensureFolderId(assignment.folderId || assignment.folderID)
        if (!deviceId || !folderId || !folderIdSet.has(folderId)) {
          return
        }
        const existingFolders = assignmentMap.get(deviceId) || []
        if (!existingFolders.includes(folderId)) {
          assignmentMap.set(deviceId, [...existingFolders, folderId])
        }
      })

      const existingByKey = new Map()
      state.devices.forEach((device) => {
        const key = device.apiId || device.id
        if (key) {
          existingByKey.set(key, device)
        }
      })

      const nextDevices = incomingDevices.map((device) => {
        const key = normalizeValue(device?.apiId || device?.id)
        const existing = key ? existingByKey.get(key) : null
        const deviceId = normalizeValue(device?.id || device?.apiId)
        const foldersForDevice = assignmentMap.get(deviceId)
        const shapedDevice = foldersForDevice && foldersForDevice.length
          ? { ...device, folderIds: foldersForDevice }
          : device
        return baseDeviceShape(shapedDevice, existing || undefined)
      })

      const localDevices = state.devices.filter((device) => device.source === 'local')

      return {
        ...state,
        folders: normalizedFolders,
        devices: [...nextDevices, ...localDevices],
        lastUpdated: Date.now(),
      }
    }
    case 'UPSERT_FOLDER': {
      const targetId = ensureFolderId(action.payload?.id)
      const existingIndex = targetId
        ? state.folders.findIndex((folder) => folder.id === targetId)
        : -1
      const existingFolder = existingIndex >= 0 ? state.folders[existingIndex] : undefined
      const normalized = normalizeFolderRecord(action.payload, existingFolder)
      if (!normalized) {
        return state
      }
      if (existingIndex >= 0) {
        const nextFolders = state.folders.slice()
        nextFolders[existingIndex] = normalized
        return { ...state, folders: nextFolders, lastUpdated: Date.now() }
      }
      return {
        ...state,
        folders: [...state.folders, normalized],
        lastUpdated: Date.now(),
      }
    }
    case 'CREATE_DEVICE': {
      const {
        name,
        channel,
        channelList,
        organization,
        folderIds: payloadFolderIds,
        folderId,
        attributes,
        remoteControlId,
      } = action.payload || {}
      const normalizedName = normalizeValue(name) || 'New Device'
      const slug = deviceSlugKey(normalizedName) || uuidv4()
      let normalizedFolderIds = []
      if (Array.isArray(payloadFolderIds)) {
        normalizedFolderIds = normalizeFolderIds(payloadFolderIds)
      } else if (payloadFolderIds) {
        normalizedFolderIds = normalizeFolderIds(payloadFolderIds)
      } else {
        normalizedFolderIds = normalizeFolderIds(folderId)
      }
      const primaryFolderId = normalizedFolderIds.length
        ? normalizedFolderIds[0]
        : null
      const device = {
        id: uuidv4(),
        apiId: null,
        name: normalizedName,
        slug,
        slugKey: deviceSlugKey(slug),
        channel: normalizeValue(channel) || '',
        channelList: normalizeValue(channelList) || '',
        organization: normalizeValue(organization) || '',
        timeZone: '',
        folderIds: normalizedFolderIds,
        folderId: primaryFolderId,
        source: 'local',
        remoteControlId: normalizeValue(remoteControlId) || '',
        attributes: attributes && typeof attributes === 'object' ? { ...attributes } : {},
      }
      return {
        ...state,
        devices: [...state.devices, device],
        lastUpdated: Date.now(),
      }
    }
    case 'UPDATE_DEVICE': {
      const {
        id,
        name,
        channel,
        channelList,
        organization,
        folderIds,
        folderId,
        remoteControlId,
        isLocked,
      } = action.payload || {}
      if (!id) {
        return state
      }
      const nextDevices = state.devices.map((device) => {
        if (device.id !== id) {
          return device
        }
        const isLocal = device.source === 'local'
        let nextFolderIds = device.folderIds || []
        if (folderIds !== undefined) {
          nextFolderIds = normalizeFolderIds(folderIds)
        } else if (folderId !== undefined) {
          nextFolderIds = normalizeFolderIds(folderId)
        }
        const primaryFolderId = nextFolderIds.length ? nextFolderIds[0] : null
        return {
          ...device,
          name: isLocal ? normalizeValue(name) || device.name : device.name,
          channel: normalizeValue(channel) || device.channel,
          channelList: normalizeValue(channelList) || device.channelList,
          organization:
            normalizeValue(organization) || device.organization,
          folderIds: nextFolderIds,
          folderId: primaryFolderId,
          remoteControlId:
            remoteControlId !== undefined
              ? normalizeValue(remoteControlId)
              : device.remoteControlId,
          isLocked:
            typeof isLocked === 'boolean' ? isLocked : device.isLocked,
        }
      })
      return { ...state, devices: nextDevices, lastUpdated: Date.now() }
    }
    case 'ASSIGN_DEVICE_FOLDER': {
      const { id, folderIds, folderId } = action.payload || {}
      if (!id) {
        return state
      }
      const normalizedFolderIds =
        folderIds !== undefined
          ? normalizeFolderIds(folderIds)
          : normalizeFolderIds(folderId)
      const nextDevices = state.devices.map((device) => {
        if (device.id !== id) {
          return device
        }
        return {
          ...device,
          folderIds: normalizedFolderIds,
          folderId: normalizedFolderIds.length ? normalizedFolderIds[0] : null,
        }
      })
      return { ...state, devices: nextDevices, lastUpdated: Date.now() }
    }
    case 'DELETE_NODES': {
      const { folderIds: rawFolderIds, deviceIds: rawDeviceIds } = action.payload || {}
      const folderIdSet = new Set(
        Array.isArray(rawFolderIds)
          ? rawFolderIds.map(ensureFolderId).filter(Boolean)
          : [],
      )
      const deviceIdSet = new Set(
        Array.isArray(rawDeviceIds)
          ? rawDeviceIds
              .map((value) => (typeof value === 'string' ? value : null))
              .filter(Boolean)
          : [],
      )

      if (folderIdSet.size === 0 && deviceIdSet.size === 0) {
        return state
      }

      const childrenByParent = new Map()
      state.folders.forEach((folder) => {
        const parent = ensureFolderId(folder.parentId)
        if (!parent) {
          return
        }
        if (!childrenByParent.has(parent)) {
          childrenByParent.set(parent, [])
        }
        childrenByParent.get(parent).push(folder.id)
      })

      const collectDescendants = (folderId) => {
        const queue = [...(childrenByParent.get(folderId) || [])]
        while (queue.length) {
          const current = queue.shift()
          if (!current || folderIdSet.has(current)) {
            continue
          }
          folderIdSet.add(current)
          const children = childrenByParent.get(current)
          if (Array.isArray(children) && children.length) {
            queue.push(...children)
          }
        }
      }

      Array.from(folderIdSet).forEach((folderId) => {
        collectDescendants(folderId)
      })

      const nextFolders = state.folders.filter(
        (folder) => !folderIdSet.has(folder.id),
      )

      const nextDevices = state.devices
        .filter((device) => !deviceIdSet.has(device.id))
        .map((device) => {
          if (!Array.isArray(device.folderIds) || device.folderIds.length === 0) {
            return device
          }
          const filteredFolderIds = device.folderIds.filter(
            (folderId) => !folderIdSet.has(folderId),
          )
          if (filteredFolderIds.length === device.folderIds.length) {
            return device
          }
          return {
            ...device,
            folderIds: filteredFolderIds,
            folderId: filteredFolderIds.length ? filteredFolderIds[0] : null,
          }
        })

      return {
        ...state,
        folders: nextFolders,
        devices: nextDevices,
        lastUpdated: Date.now(),
      }
    }
    default:
      return state
  }
}

const buildTree = (folders, devices) => {
  const folderMap = new Map()
  const rootNodes = []

  folders.forEach((folder) => {
    folderMap.set(folder.id, {
      ...folder,
      type: 'folder',
      children: [],
    })
  })

  folderMap.forEach((folderNode) => {
    if (folderNode.parentId && folderMap.has(folderNode.parentId)) {
      const parent = folderMap.get(folderNode.parentId)
      parent.children.push(folderNode)
    } else {
      rootNodes.push(folderNode)
    }
  })

  const attachDevice = (device) => {
    const folderIds = normalizeFolderIds(device.folderIds || device.folderId)
    if (!folderIds.length) {
      rootNodes.push({
        ...device,
        type: 'device',
        treeKey: `${device.id}-root`,
      })
      return
    }
    const uniqueIds = Array.from(new Set(folderIds))
    uniqueIds.forEach((folderId) => {
      const node = {
        ...device,
        type: 'device',
        treeKey: `${device.id}-${folderId}`,
      }
      if (folderMap.has(folderId)) {
        folderMap.get(folderId).children.push(node)
      } else {
        rootNodes.push(node)
      }
    })
  }

  devices.forEach(attachDevice)

  const sortNodes = (nodes) =>
    nodes
      .slice()
      .sort((a, b) => {
        if (a.type === b.type) {
          return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
        }
        return a.type === 'folder' ? -1 : 1
      })

  const normalizeTree = (nodes) =>
    sortNodes(nodes).map((node) => {
      if (node.type === 'folder') {
        return {
          ...node,
          children: normalizeTree(node.children || []),
        }
      }
      return node
    })

  return normalizeTree(rootNodes)
}

const RetailPlayerDeviceStoreProvider = ({ children }) => {
  const [state, dispatch] = useReducer(reducer, initialState)
  const {
    devices: remoteDevices,
    folders: remoteFolders,
    deviceFolders: remoteDeviceFolders,
    error,
    isLoading,
    isApiEnabled,
  } = useRetailPlayerDevices()

  useEffect(() => {
    dispatch({ type: 'SET_LOADING', payload: isLoading })
  }, [isLoading])

  useEffect(() => {
    dispatch({ type: 'SET_ERROR', payload: error })
  }, [error])

  useEffect(() => {
    dispatch({ type: 'SET_API_ENABLED', payload: isApiEnabled })
  }, [isApiEnabled])

  useEffect(() => {
    if (
      !Array.isArray(remoteDevices) &&
      !Array.isArray(remoteFolders) &&
      !Array.isArray(remoteDeviceFolders)
    ) {
      return
    }
    dispatch({
      type: 'SYNC_REMOTE',
      payload: {
        devices: Array.isArray(remoteDevices) ? remoteDevices : [],
        folders: Array.isArray(remoteFolders) ? remoteFolders : [],
        deviceFolders: Array.isArray(remoteDeviceFolders)
          ? remoteDeviceFolders
          : [],
      },
    })
  }, [remoteDevices, remoteFolders, remoteDeviceFolders])

  const tree = useMemo(
    () => buildTree(state.folders, state.devices),
    [state.folders, state.devices],
  )

  const apiEnabled = state.isApiEnabled

  const createFolder = useCallback(
    async (payload) => {
      const basePayload = payload && typeof payload === 'object' ? payload : {}
      if (!apiEnabled) {
        const folderPayload = normalizeFolderRecord({
          ...basePayload,
          id: ensureFolderId(basePayload.id) || uuidv4(),
        })
        if (folderPayload) {
          dispatch({ type: 'UPSERT_FOLDER', payload: folderPayload })
        }
        return folderPayload
      }

      const requestBody = {
        name: normalizeValue(basePayload.name) || 'New Folder',
      }
      const providedId = ensureFolderId(basePayload.id)
      if (providedId) {
        requestBody.id = providedId
      }
      if (Object.prototype.hasOwnProperty.call(basePayload, 'parentId')) {
        const normalizedParent = ensureFolderId(basePayload.parentId)
        requestBody.parentId = normalizedParent || null
      }

      const { json } = await httpClient('/api/retailplayer/folders', {
        method: 'POST',
        body: JSON.stringify(requestBody),
        headers: new Headers({ 'Content-Type': 'application/json' }),
      })

      const folder = normalizeFolderRecord(json?.data)
      if (folder) {
        dispatch({ type: 'UPSERT_FOLDER', payload: folder })
      }
      return folder
    },
    [apiEnabled, dispatch],
  )

  const updateFolder = useCallback(
    async (payload) => {
      const basePayload = payload && typeof payload === 'object' ? payload : {}
      const folderId = ensureFolderId(basePayload.id)
      if (!folderId) {
        return null
      }

      if (!apiEnabled) {
        const normalized = normalizeFolderRecord(basePayload)
        if (normalized) {
          dispatch({ type: 'UPSERT_FOLDER', payload: normalized })
        }
        return normalized
      }

      const requestBody = {}
      if (Object.prototype.hasOwnProperty.call(basePayload, 'name')) {
        const normalizedName = normalizeValue(basePayload.name)
        requestBody.name = normalizedName || 'New Folder'
      }
      if (Object.prototype.hasOwnProperty.call(basePayload, 'parentId')) {
        const normalizedParent = ensureFolderId(basePayload.parentId)
        requestBody.parentId = normalizedParent || null
      }

      const { json } = await httpClient(
        `/api/retailplayer/folders/${encodeURIComponent(folderId)}`,
        {
          method: 'PATCH',
          body: JSON.stringify(requestBody),
          headers: new Headers({ 'Content-Type': 'application/json' }),
        },
      )

      const folder = normalizeFolderRecord(json?.data)
      if (folder) {
        dispatch({ type: 'UPSERT_FOLDER', payload: folder })
      }
      return folder
    },
    [apiEnabled, dispatch],
  )

  const createDevice = useCallback((payload) => {
    dispatch({ type: 'CREATE_DEVICE', payload })
  }, [])

  const updateDevice = useCallback(
    async (payload) => {
      const basePayload = payload && typeof payload === 'object' ? payload : {}
      dispatch({ type: 'UPDATE_DEVICE', payload: basePayload })

      if (!apiEnabled) {
        return basePayload
      }

      const deviceId = typeof basePayload.id === 'string' ? basePayload.id : null
      if (!deviceId) {
        return basePayload
      }

      const hasRemoteControlId = Object.prototype.hasOwnProperty.call(
        basePayload,
        'remoteControlId',
      )
      const hasIsLocked = Object.prototype.hasOwnProperty.call(
        basePayload,
        'isLocked',
      )

      if (hasRemoteControlId) {
        const remoteControlId = normalizeValue(basePayload.remoteControlId)
        const { json } = await httpClient(
          `/api/retailplayer/devices/${encodeURIComponent(deviceId)}/remote-control`,
          {
            method: 'PATCH',
            body: JSON.stringify({ remoteControlId }),
            headers: new Headers({ 'Content-Type': 'application/json' }),
          },
        )

        const normalizedRemoteControlId = normalizeValue(
          json?.data?.remoteControlId ?? remoteControlId,
        )
        if (normalizedRemoteControlId !== undefined) {
          dispatch({
            type: 'UPDATE_DEVICE',
            payload: { id: deviceId, remoteControlId: normalizedRemoteControlId },
          })
        }
      }

      if (hasIsLocked) {
        const isLocked = Boolean(basePayload.isLocked)
        const { json } = await httpClient(
          `/api/retailplayer/devices/${encodeURIComponent(deviceId)}/lock`,
          {
            method: 'PATCH',
            body: JSON.stringify({ locked: isLocked }),
            headers: new Headers({ 'Content-Type': 'application/json' }),
          },
        )

        dispatch({
          type: 'UPDATE_DEVICE',
          payload: {
            id: deviceId,
            isLocked:
              typeof json?.data?.isLocked === 'boolean'
                ? json.data.isLocked
                : isLocked,
          },
        })
      }

      const hasFolderIds = Object.prototype.hasOwnProperty.call(
        basePayload,
        'folderIds',
      )
      const hasFolderId = Object.prototype.hasOwnProperty.call(
        basePayload,
        'folderId',
      )

      if (!hasFolderIds && !hasFolderId) {
        return basePayload
      }

      const normalizedFolderIds = hasFolderIds
        ? normalizeFolderIds(basePayload.folderIds)
        : normalizeFolderIds(basePayload.folderId)

      await httpClient(
        `/api/retailplayer/devices/${encodeURIComponent(deviceId)}/folders`,
        {
          method: 'PUT',
          body: JSON.stringify({ folderIds: normalizedFolderIds }),
          headers: new Headers({ 'Content-Type': 'application/json' }),
        },
      )

      dispatch({
        type: 'ASSIGN_DEVICE_FOLDER',
        payload: { id: deviceId, folderIds: normalizedFolderIds },
      })

      return basePayload
    },
    [apiEnabled, dispatch],
  )

  const assignDeviceToFolder = useCallback(
    async (payload) => {
      const basePayload = payload && typeof payload === 'object' ? payload : {}
      const deviceId = typeof basePayload.id === 'string' ? basePayload.id : null
      if (!deviceId) {
        return []
      }

      const normalizedFolderIds = Object.prototype.hasOwnProperty.call(
        basePayload,
        'folderIds',
      )
        ? normalizeFolderIds(basePayload.folderIds)
        : normalizeFolderIds(basePayload.folderId)

      if (!apiEnabled) {
        dispatch({
          type: 'ASSIGN_DEVICE_FOLDER',
          payload: { id: deviceId, folderIds: normalizedFolderIds },
        })
        return normalizedFolderIds
      }

      await httpClient(
        `/api/retailplayer/devices/${encodeURIComponent(deviceId)}/folders`,
        {
          method: 'PUT',
          body: JSON.stringify({ folderIds: normalizedFolderIds }),
          headers: new Headers({ 'Content-Type': 'application/json' }),
        },
      )

      dispatch({
        type: 'ASSIGN_DEVICE_FOLDER',
        payload: { id: deviceId, folderIds: normalizedFolderIds },
      })

      return normalizedFolderIds
    },
    [apiEnabled, dispatch],
  )

  const deleteNodes = useCallback(
    async (payload) => {
      const basePayload = payload && typeof payload === 'object' ? payload : {}
      const folderIds = Array.isArray(basePayload.folderIds)
        ? basePayload.folderIds.map(ensureFolderId).filter(Boolean)
        : []
      const deviceIds = Array.isArray(basePayload.deviceIds)
        ? basePayload.deviceIds
            .map((value) => (typeof value === 'string' ? value : null))
            .filter(Boolean)
        : []

      if (apiEnabled && folderIds.length) {
        await httpClient('/api/retailplayer/folders/delete', {
          method: 'POST',
          body: JSON.stringify({ folderIds }),
          headers: new Headers({ 'Content-Type': 'application/json' }),
        })
      }

      dispatch({
        type: 'DELETE_NODES',
        payload: { folderIds, deviceIds },
      })
    },
    [apiEnabled, dispatch],
  )

  const value = useMemo(
    () => ({
      state: { ...state, tree },
      actions: {
        createFolder,
        updateFolder,
        createDevice,
        updateDevice,
        assignDeviceToFolder,
        deleteNodes,
      },
    }),
    [
      state,
      tree,
      createFolder,
      updateFolder,
      createDevice,
      updateDevice,
      assignDeviceToFolder,
      deleteNodes,
    ],
  )

  return (
    <RetailPlayerDeviceStoreContext.Provider value={value}>
      {children}
    </RetailPlayerDeviceStoreContext.Provider>
  )
}

RetailPlayerDeviceStoreProvider.propTypes = {
  children: PropTypes.node.isRequired,
}

const useRetailPlayerDeviceStore = () => {
  const context = useContext(RetailPlayerDeviceStoreContext)
  if (!context) {
    throw new Error(
      'useRetailPlayerDeviceStore must be used within RetailPlayerDeviceStoreProvider',
    )
  }
  return context
}

// eslint-disable-next-line react-refresh/only-export-components
export { useRetailPlayerDeviceStore }

export default RetailPlayerDeviceStoreProvider
