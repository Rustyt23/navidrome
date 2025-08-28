import jsonServerProvider from 'ra-data-json-server'
import httpClient from './httpClient'
import { REST_URL } from '../consts'

const dataProvider = jsonServerProvider(REST_URL, httpClient)

const isAdmin = () => {
  const role = localStorage.getItem('role')
  return role === 'admin'
}

const mapResource = (resource, params) => {
  switch (resource) {
    case 'playlistTrack': {
      // /api/playlistTrack?playlist_id=123  => /api/playlist/123/tracks
      let plsId = '0'
      if (params.filter) {
        plsId = params.filter.playlist_id
        if (!isAdmin()) {
          params.filter.missing = false
        }
      }
      return [`playlist/${plsId}/tracks`, params]
    }
    case 'album':
    case 'song': {
      if (params.filter && !isAdmin()) {
        params.filter.missing = false
      }
      return [resource, params]
    }
    default:
      return [resource, params]
  }
}

const callDeleteMany = (resource, params) => {
  const ids = params.ids.map((id) => `id=${id}`)
  const idsParam = ids.join('&')
  return httpClient(`${REST_URL}/${resource}?${idsParam}`, {
    method: 'DELETE',
  }).then((response) => ({ data: response.json.ids || [] }))
}

const emitFoldersChanged = (detail) => {
  try {
    window.dispatchEvent(new CustomEvent('folder:changed', { detail }))
  } catch {}
}

const wrapperDataProvider = {
  ...dataProvider,
  getList: (resource, params) => {
    const [r, p] = mapResource(resource, params)
    return dataProvider.getList(r, p)
  },
  getOne: (resource, params) => {
    const [r, p] = mapResource(resource, params)
    return dataProvider.getOne(r, p)
  },
  getMany: (resource, params) => {
    const [r, p] = mapResource(resource, params)
    return dataProvider.getMany(r, p)
  },
  getManyReference: (resource, params) => {
    const [r, p] = mapResource(resource, params)
    return dataProvider.getManyReference(r, p)
  },
  update: (resource, params) => {
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
    if (r.endsWith('/tracks') || resource === 'missing') {
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
}

export default wrapperDataProvider
