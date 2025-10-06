import jsonServerProvider from 'ra-data-json-server'
import httpClient from './httpClient'
import { REST_URL } from '../consts'

const dataProvider = jsonServerProvider(REST_URL, httpClient)

const isAdmin = () => {
  const role = localStorage.getItem('role')
  return role === 'admin'
}

const getSelectedLibraries = () => {
  try {
    const state = JSON.parse(localStorage.getItem('state'))
    return state?.library?.selectedLibraries || []
  } catch (err) {
    return []
  }
}

// Function to apply library filtering to appropriate resources
const applyLibraryFilter = (resource, params) => {
  // Content resources that should be filtered by selected libraries
  const filteredResources = ['album', 'song', 'artist', 'playlistTrack', 'tag']

  // Get selected libraries from localStorage
  const selectedLibraries = getSelectedLibraries()

  // Add library filter for content resources if libraries are selected
  if (filteredResources.includes(resource) && selectedLibraries.length > 0) {
    if (!params.filter) {
      params.filter = {}
    }
    params.filter.library_id = selectedLibraries
  }

  return params
}

const mapResource = (resource, params) => {
  switch (resource) {
    // /api/playlistTrack?playlist_id=123  => /api/playlist/123/tracks
    case 'playlistTrack': {
      params.filter = params.filter || {}

      let plsId = '0'
      plsId = params.filter.playlist_id
      params = applyLibraryFilter(resource, params)

      return [`playlist/${plsId}/tracks`, params]
    }
    case 'album':
    case 'song':
    case 'artist':
    case 'tag': {
      params.filter = params.filter || {}
      if (!isAdmin()) {
        params.filter.missing = false
      }
      params = applyLibraryFilter(resource, params)

      return [resource, params]
    }
    default:
      return [resource, params]
  }
}

const callDeleteMany = (resource, params) => {
  const ids = (params.ids || []).map((id) => `id=${id}`)
  const query = ids.length > 0 ? `?${ids.join('&')}` : ''
  return httpClient(`${REST_URL}/${resource}${query}`, {
    method: 'DELETE',
  }).then((response) => ({ data: response.json.ids || [] }))
}

// Helper function to handle user-library associations
const handleUserLibraryAssociation = async (userId, libraryIds) => {
  if (!libraryIds || libraryIds.length === 0) {
    return // Admin users or users without library assignments
  }

  try {
    await httpClient(`${REST_URL}/user/${userId}/library`, {
      method: 'PUT',
      body: JSON.stringify({ libraryIds }),
    })
  } catch (error) {
    console.error('Error setting user libraries:', error) //eslint-disable-line no-console
    throw error
  }
}

const sortDiscoveryRecords = (records, sort) => {
  if (!sort?.field) return [...records]
  const { field, order } = sort
  const direction = order === 'DESC' ? -1 : 1
  return [...records].sort((a, b) => {
    const aValue = a?.[field]
    const bValue = b?.[field]
    if (aValue === undefined || aValue === null) {
      return bValue === undefined || bValue === null ? 0 : -1 * direction
    }
    if (bValue === undefined || bValue === null) {
      return 1 * direction
    }
    if (typeof aValue === 'number' && typeof bValue === 'number') {
      return (aValue - bValue) * direction
    }
    if (typeof aValue === 'string' && typeof bValue === 'string') {
      return aValue.localeCompare(bValue) * direction
    }
    const aText = `${aValue}`
    const bText = `${bValue}`
    const aNumber = Number(aText)
    const bNumber = Number(bText)
    if (!Number.isNaN(aNumber) && !Number.isNaN(bNumber)) {
      return (aNumber - bNumber) * direction
    }
    return aText.localeCompare(bText) * direction
  })
}

const applyDiscoveryFilter = (records, filter) => {
  if (!filter?.q) return [...records]
  const query = String(filter.q).trim().toLowerCase()
  if (!query) return [...records]
  return records.filter((record) =>
    (record?.name || '').toLowerCase().includes(query)
  )
}

const applyDiscoveryTrackFilter = (records, filter = {}) => {
  const query = String(filter.q || '').trim().toLowerCase()
  if (!query) {
    return [...records]
  }
  return records.filter((track) => {
    const candidates = [
      track?.title,
      track?.album,
      track?.artist,
      track?.albumArtist,
      track?.path,
    ]
    return candidates.some((value) =>
      String(value || '')
        .toLowerCase()
        .includes(query),
    )
  })
}

const getDiscoveryList = async (params = {}) => {
  const { pagination = {}, sort, filter } = params
  const { json } = await httpClient(`${REST_URL}/discovery`)
  const records = Array.isArray(json) ? json : []
  const filtered = applyDiscoveryFilter(records, filter)
  const sorted = sortDiscoveryRecords(filtered, sort)
  const perPageRaw = pagination.perPage
  const perPage =
    perPageRaw === 0
      ? sorted.length || 1
      : Math.max(1, perPageRaw || (sorted.length || 1))
  const page = Math.max(1, pagination.page || 1)
  const start = (page - 1) * perPage
  const end = perPageRaw === 0 ? sorted.length : start + perPage
  const paginated = sorted.slice(start, end)
  return { data: paginated, total: sorted.length }
}

const getDiscoveryOne = async (id) => {
  const { json } = await httpClient(`${REST_URL}/discovery/${id}`)
  return { data: json }
}

const findDiscoveryTrack = async (trackId, discoveryId) => {
  if (!trackId) {
    return null
  }

  const searchWithin = async (id) => {
    if (!id) {
      return null
    }
    const res = await getDiscoveryTracks({
      filter: { discovery_id: id },
      pagination: { page: 1, perPage: 0 },
    })
    return res.data.find((item) => item.id === trackId) || null
  }

  const direct = await searchWithin(discoveryId)
  if (direct) {
    return direct
  }

  const all = await getDiscoveryList({
    pagination: { page: 1, perPage: 0 },
  })

  for (const playlist of all.data) {
    const found = await searchWithin(playlist.id)
    if (found) {
      return found
    }
  }

  return null
}

const sortDiscoveryTracks = (records, sort) => {
  if (!sort?.field) {
    return [...records]
  }
  const { field, order } = sort
  const direction = order === 'DESC' ? -1 : 1
  return [...records].sort((a, b) => {
    const aValue = a?.[field]
    const bValue = b?.[field]
    if (typeof aValue === 'number' && typeof bValue === 'number') {
      return (aValue - bValue) * direction
    }
    const aText = String(aValue ?? '').toLowerCase()
    const bText = String(bValue ?? '').toLowerCase()
    return aText.localeCompare(bText) * direction
  })
}

const getDiscoveryTracks = async (params = {}) => {
  const { pagination = {}, sort, filter = {} } = params
  const discoveryId =
    filter.discovery_id || filter.discoveryId || filter.id || ''
  if (!discoveryId) {
    return { data: [], total: 0 }
  }
  const { json } = await httpClient(
    `${REST_URL}/discovery/${discoveryId}/tracks`,
  )
  const records = Array.isArray(json) ? json : []
  const filtered = applyDiscoveryTrackFilter(records, filter)
  const sorted = sortDiscoveryTracks(filtered, sort)
  const total = sorted.length
  const perPageRaw = pagination.perPage
  const perPage =
    perPageRaw === 0
      ? total || 1
      : Math.max(1, perPageRaw || (sorted.length || 1))
  const page = Math.max(1, pagination.page || 1)
  const start = (page - 1) * perPage
  const end = perPageRaw === 0 ? sorted.length : start + perPage
  const paginated = sorted.slice(start, end)
  return { data: paginated, total }
}

// Enhanced user creation that handles library associations
const createUser = async (params) => {
  const { data } = params
  const { libraryIds, ...userData } = data

  // First create the user
  const userResponse = await dataProvider.create('user', { data: userData })
  const userId = userResponse.data.id

  // Then set library associations for non-admin users
  if (!userData.isAdmin && libraryIds && libraryIds.length > 0) {
    await handleUserLibraryAssociation(userId, libraryIds)
  }

  return userResponse
}

// Enhanced user update that handles library associations
const updateUser = async (params) => {
  const { data } = params
  const { libraryIds, ...userData } = data
  const userId = params.id

  // First update the user
  const userResponse = await dataProvider.update('user', {
    ...params,
    data: userData,
  })

  // Then handle library associations for non-admin users
  if (!userData.isAdmin && libraryIds !== undefined) {
    await handleUserLibraryAssociation(userId, libraryIds)
  }

  return userResponse
}

const emitFoldersChanged = (detail) => {
  try {
    window.dispatchEvent(new CustomEvent('folder:changed', { detail }))
  } catch (err) {
    // Ignore errors if dispatching fails
  }
}

const wrapperDataProvider = {
  ...dataProvider,
  getList: (resource, params) => {
    if (resource === 'discovery') {
      return getDiscoveryList(params)
    }
    if (resource === 'discoveryTrack') {
      return getDiscoveryTracks(params)
    }
    const [r, p] = mapResource(resource, params)
    return dataProvider.getList(r, p)
  },
  getOne: (resource, params) => {
    if (resource === 'discovery') {
      return getDiscoveryOne(params.id)
    }
    if (resource === 'discoveryTrack') {
      return findDiscoveryTrack(params.id, params?.filter?.discovery_id).then(
        (track) => ({ data: track }),
      )
    }
    const [r, p] = mapResource(resource, params)
    const response = dataProvider.getOne(r, p)

    // Transform user data to ensure libraryIds is present for form compatibility
    if (resource === 'user') {
      return response.then((result) => {
        if (result.data.libraries && Array.isArray(result.data.libraries)) {
          result.data.libraryIds = result.data.libraries.map((lib) => lib.id)
        }
        return result
      })
    }

    return response
  },
  getMany: (resource, params) => {
    if (resource === 'discovery') {
      const ids = params.ids || []
      return Promise.all(ids.map((id) => getDiscoveryOne(id))).then((results) => ({
        data: results.map((item) => item.data),
      }))
    }
    if (resource === 'discoveryTrack') {
      const filter = params?.filter || {}
      return getDiscoveryTracks({
        filter,
        pagination: { page: 1, perPage: 0 },
      }).then((res) => ({
        data: res.data.filter((track) => params.ids.includes(track.id)),
      }))
    }
    const [r, p] = mapResource(resource, params)
    return dataProvider.getMany(r, p)
  },
  getManyReference: (resource, params) => {
    if (resource === 'discovery') {
      return getDiscoveryList(params)
    }
    if (resource === 'discoveryTrack') {
      return getDiscoveryTracks(params)
    }
    const [r, p] = mapResource(resource, params)
    return dataProvider.getManyReference(r, p)
  },
  update: (resource, params) => {
    if (resource === 'user') {
      return updateUser(params)
    }
    const [r, p] = mapResource(resource, params)
    return dataProvider.update(r, p).then((res) => {
      if (resource === 'playlist' || resource === 'folder') {
        const parentId =
          (params?.data?.folderId ?? params?.data?.parentId ?? '') || ''
        emitFoldersChanged({ type: 'create', resource, targetParentId: parentId })
      }
      return res
    })
  },
  updateMany: (resource, params) => {
    const [r, p] = mapResource(resource, params)
    return dataProvider.updateMany(r, p)
  },
  create: (resource, params) => {
    if (resource === 'user') {
      return createUser(params)
    }
    const [r, p] = mapResource(resource, params)
    return dataProvider.create(r, p).then((res) => {
      if (resource === 'playlist' || resource === 'folder') {
        const parentId =
          (params?.data?.folderId ?? params?.data?.parentId ?? '') || ''
        emitFoldersChanged({ type: 'create', resource, targetParentId: parentId })
      }
      return res
    })
  },
  delete: (resource, params) => {
    const [r, p] = mapResource(resource, params)
    return dataProvider.delete(r, p).then((res) => {
      if (resource === 'playlist' || resource === 'folder') {
        emitFoldersChanged({ type: 'delete', resource, targetParentId: '' })
      }
      return res
    })
  },
  deleteMany: (resource, params) => {
    const [r, p] = mapResource(resource, params)
    if (r.endsWith('/tracks') || resource === 'missing' || resource === 'folder') {
      return callDeleteMany(r, p)
    }
    return dataProvider.deleteMany(r, p)
  },
  addToPlaylist: (playlistId, data) => {
    return httpClient(`${REST_URL}/playlist/${playlistId}/tracks`, {
      method: 'POST',
      body: JSON.stringify(data),
    }).then(({ json }) => ({ data: json }))
  },
  getPlaylists: (songId) => {
    return httpClient(`${REST_URL}/song/${songId}/playlists`).then(
      ({ json }) => ({ data: json }),
    )
  },
  inspect: (songId) => {
    return httpClient(`${REST_URL}/inspect?id=${songId}`).then(({ json }) => ({
      data: json,
    }))
  },

  setPlaylistFolder: ({ playlistId, targetFolderId, sourceParentId }) => {
    return httpClient(`${REST_URL}/playlist/${playlistId}/folder`, {
      method: 'PATCH',
      body: JSON.stringify({
        folderId: targetFolderId
      }),
    }).then(() => {
      emitFoldersChanged({
        type: 'move',
        resource: 'playlist',
        sourceParentId: sourceParentId ?? '',
        targetParentId: targetFolderId ?? '',
      })
      return { data: { id: playlistId, folderId: targetFolderId } }
    })
  },

  moveFolder: ({ folderId, targetParentId, sourceParentId }) => {
    return httpClient(`${REST_URL}/folder/${folderId}/parent`, {
      method: 'PATCH',
      body: JSON.stringify({
        parentId: targetParentId
      }),
    }).then(({ json }) => {
      emitFoldersChanged({
        type: 'move',
        resource: 'folder',
        sourceParentId: sourceParentId ?? '',
        targetParentId: targetParentId ?? '',
      })
      return { data: json }
    })
  },

  bulkMove: ({ playlistIds = [], folderIds = [], targetParentId = null }) => {
    return httpClient(`${REST_URL}/folder/move`, {
      method: 'PATCH',
      body: JSON.stringify({ playlistIds, folderIds, targetParentId }),
    }).then(({ json }) => {
      emitFoldersChanged({
        type: 'bulkMove',
        targetParentId: targetParentId ?? '',
      })
      if ((targetParentId ?? '') === '') {
        emitFoldersChanged({ type: 'bulkMove', targetParentId: '' })
      }
      return { data: json }
    })
  },
  refreshDiscovery: () => {
    return httpClient(`${REST_URL}/discovery/refresh`, {
      method: 'POST',
    }).then(({ json }) => ({ data: json }))
  },
  publishDiscovery: (id) => {
    return httpClient(`${REST_URL}/discovery/${id}/publish`, {
      method: 'POST',
    }).then(() => ({ data: { id } }))
  },
}

export default wrapperDataProvider
