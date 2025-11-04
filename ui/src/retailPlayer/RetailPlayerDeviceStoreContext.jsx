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

const baseDeviceShape = (device, existing) => {
  const normalizedName = normalizeValue(device?.name)
  const normalizedSlug = normalizeValue(device?.slug)
  const normalizedChannel = normalizeValue(device?.channel)
  const normalizedChannelList = normalizeValue(device?.channelList)
  const normalizedOrganization = normalizeValue(device?.organization)
  const normalizedTimeZone = normalizeValue(device?.timeZone)

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
    organization: normalizedOrganization || '',
    timeZone: normalizedTimeZone || '',
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
    case 'SYNC_DEVICES': {
      const incoming = Array.isArray(action.payload) ? action.payload : []
      const existingByKey = new Map()
      state.devices.forEach((device) => {
        const key = device.apiId || device.id
        if (key) {
          existingByKey.set(key, device)
        }
      })

      const nextDevices = incoming.map((device) => {
        const key = normalizeValue(device?.apiId || device?.id)
        const existing = key ? existingByKey.get(key) : null
        return baseDeviceShape(device, existing || undefined)
      })

      const localDevices = state.devices.filter((device) => device.source === 'local')

      return {
        ...state,
        devices: [...nextDevices, ...localDevices],
        lastUpdated: Date.now(),
      }
    }
    case 'CREATE_FOLDER': {
      const { name, parentId } = action.payload || {}
      const now = new Date().toISOString()
      const folder = {
        id: uuidv4(),
        name: normalizeValue(name) || 'New Folder',
        parentId: ensureFolderId(parentId),
        createdAt: now,
        updatedAt: now,
      }
      return {
        ...state,
        folders: [...state.folders, folder],
        lastUpdated: Date.now(),
      }
    }
    case 'UPDATE_FOLDER': {
      const { id, name } = action.payload || {}
      if (!id) {
        return state
      }
      const nextFolders = state.folders.map((folder) => {
        if (folder.id !== id) {
          return folder
        }
        return {
          ...folder,
          name: normalizeValue(name) || folder.name,
          updatedAt: new Date().toISOString(),
        }
      })
      return { ...state, folders: nextFolders, lastUpdated: Date.now() }
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
    if (Array.isArray(remoteDevices)) {
      dispatch({ type: 'SYNC_DEVICES', payload: remoteDevices })
    }
  }, [remoteDevices])

  const tree = useMemo(
    () => buildTree(state.folders, state.devices),
    [state.folders, state.devices],
  )

  const createFolder = useCallback((payload) => {
    dispatch({ type: 'CREATE_FOLDER', payload })
  }, [])

  const updateFolder = useCallback((payload) => {
    dispatch({ type: 'UPDATE_FOLDER', payload })
  }, [])

  const createDevice = useCallback((payload) => {
    dispatch({ type: 'CREATE_DEVICE', payload })
  }, [])

  const updateDevice = useCallback((payload) => {
    dispatch({ type: 'UPDATE_DEVICE', payload })
  }, [])

  const assignDeviceToFolder = useCallback((payload) => {
    dispatch({ type: 'ASSIGN_DEVICE_FOLDER', payload })
  }, [])

  const value = useMemo(
    () => ({
      state: { ...state, tree },
      actions: {
        createFolder,
        updateFolder,
        createDevice,
        updateDevice,
        assignDeviceToFolder,
      },
    }),
    [state, tree, createFolder, updateFolder, createDevice, updateDevice, assignDeviceToFolder],
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

export { RetailPlayerDeviceStoreProvider, useRetailPlayerDeviceStore }

export default RetailPlayerDeviceStoreProvider
