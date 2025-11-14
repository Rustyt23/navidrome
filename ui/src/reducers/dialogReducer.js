import {
  ADD_TO_PLAYLIST_CLOSE,
  ADD_TO_PLAYLIST_OPEN,
  DOWNLOAD_MENU_ALBUM,
  DOWNLOAD_MENU_ARTIST,
  DOWNLOAD_MENU_CLOSE,
  DOWNLOAD_MENU_OPEN,
  DOWNLOAD_MENU_PLAY,
  DOWNLOAD_MENU_SONG,
  DUPLICATE_SONG_WARNING_OPEN,
  DUPLICATE_SONG_WARNING_CLOSE,
  EDIT_COMMENTS_OPEN,
  EDIT_COMMENTS_CLOSE,
  EXTENDED_INFO_OPEN,
  EXTENDED_INFO_CLOSE,
  LISTENBRAINZ_TOKEN_OPEN,
  LISTENBRAINZ_TOKEN_CLOSE,
  SAVE_QUEUE_OPEN,
  SAVE_QUEUE_CLOSE,
  SHARE_MENU_OPEN,
  SHARE_MENU_CLOSE,
} from '../actions'

export const shareDialogReducer = (
  previousState = {
    open: false,
    ids: [],
    resource: '',
    name: '',
  },
  payload,
) => {
  const { type, ids, resource, name, label } = payload
  switch (type) {
    case SHARE_MENU_OPEN:
      return {
        ...previousState,
        open: true,
        ids,
        resource,
        name,
        label,
      }
    case SHARE_MENU_CLOSE:
      return {
        ...previousState,
        open: false,
      }
    default:
      return previousState
  }
}

export const addToPlaylistDialogReducer = (
  previousState = {
    open: false,
    duplicateSong: false,
  },
  payload,
) => {
  const { type } = payload
  switch (type) {
    case ADD_TO_PLAYLIST_OPEN:
      return {
        ...previousState,
        open: true,
        selectedIds: payload.selectedIds,
        onSuccess: payload.onSuccess,
      }
    case ADD_TO_PLAYLIST_CLOSE:
      return { ...previousState, open: false, onSuccess: undefined }
    case DUPLICATE_SONG_WARNING_OPEN:
      return {
        ...previousState,
        duplicateSong: true,
        duplicateIds: payload.duplicateIds,
      }
    case DUPLICATE_SONG_WARNING_CLOSE:
      return { ...previousState, duplicateSong: false }
    default:
      return previousState
  }
}

export const downloadMenuDialogReducer = (
  previousState = {
    open: false,
  },
  payload,
) => {
  const { type } = payload
  switch (type) {
    case DOWNLOAD_MENU_OPEN: {
      switch (payload.recordType) {
        case DOWNLOAD_MENU_ALBUM:
        case DOWNLOAD_MENU_ARTIST:
        case DOWNLOAD_MENU_PLAY:
        case DOWNLOAD_MENU_SONG: {
          return {
            ...previousState,
            open: true,
            record: payload.record,
            recordType: payload.recordType,
          }
        }
        default: {
          return {
            ...previousState,
            open: true,
            record: payload.record,
            recordType: undefined,
          }
        }
      }
    }
    case DOWNLOAD_MENU_CLOSE: {
      return {
        ...previousState,
        open: false,
        recordType: undefined,
      }
    }
    default:
      return previousState
  }
}

export const expandInfoDialogReducer = (
  previousState = {
    open: false,
    record: undefined,
  },
  payload,
) => {
  const { type } = payload
  switch (type) {
    case EXTENDED_INFO_OPEN:
      return {
        ...previousState,
        open: true,
        record: payload.record,
      }
    case EXTENDED_INFO_CLOSE:
      return {
        ...previousState,
        open: false,
        record: undefined,
      }
    default:
      return previousState
  }
}

export const listenBrainzTokenDialogReducer = (
  previousState = {
    open: false,
  },
  payload,
) => {
  const { type } = payload
  switch (type) {
    case LISTENBRAINZ_TOKEN_OPEN:
      return {
        ...previousState,
        open: true,
      }
    case LISTENBRAINZ_TOKEN_CLOSE:
      return {
        ...previousState,
        open: false,
      }
    default:
      return previousState
  }
}

export const saveQueueDialogReducer = (
  previousState = { open: false },
  payload,
) => {
  const { type } = payload
  switch (type) {
    case SAVE_QUEUE_OPEN:
      return { ...previousState, open: true }
    case SAVE_QUEUE_CLOSE:
      return { ...previousState, open: false }
    default:
      return previousState
  }
}

export const editCommentsDialogReducer = (
  previousState = {
    open: false,
    resource: undefined,
    selectedIds: [],
    targetIds: [],
    initialComment: '',
  },
  payload,
) => {
  const { type } = payload
  switch (type) {
    case EDIT_COMMENTS_OPEN:
      return {
        ...previousState,
        open: true,
        resource: payload.resource,
        selectedIds: payload.selectedIds || [],
        targetIds:
          payload.targetIds || payload.selectedIds || previousState.targetIds,
        initialComment: payload.initialComment || '',
        onSuccess: payload.onSuccess,
      }
    case EDIT_COMMENTS_CLOSE:
      return {
        ...previousState,
        open: false,
        selectedIds: [],
        targetIds: [],
        initialComment: '',
        onSuccess: undefined,
      }
    default:
      return previousState
  }
}
