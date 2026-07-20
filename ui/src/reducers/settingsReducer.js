import {
  SET_NOTIFICATIONS_STATE,
  SET_OMITTED_FIELDS,
  SET_TOGGLEABLE_FIELDS,
  SET_COLUMNS_ORDER,
  SET_APPBAR_ICONS,
  SET_MUSICBRAINZ_VISIBLE,
} from '../actions'

const initialState = {
  notifications: false,
  toggleableFields: {},
  omittedFields: {},
  columnsOrder: {},
  appBarIcons: {
    nowPlaying: true,
    missingTracks: true,
  },
  showMusicBrainz: true,
}

export const settingsReducer = (previousState = initialState, payload) => {
  const { type, data } = payload
  switch (type) {
    case SET_NOTIFICATIONS_STATE:
      return {
        ...previousState,
        notifications: data,
      }
    case SET_APPBAR_ICONS:
      return {
        ...previousState,
        appBarIcons: {
          ...initialState.appBarIcons,
          ...previousState.appBarIcons,
          ...data,
        },
      }
    case SET_MUSICBRAINZ_VISIBLE:
      return {
        ...previousState,
        showMusicBrainz: data,
      }
    case SET_TOGGLEABLE_FIELDS:
      return {
        ...previousState,
        toggleableFields: {
          ...previousState.toggleableFields,
          ...data,
        },
      }
    case SET_OMITTED_FIELDS:
      return {
        ...previousState,
        omittedFields: {
          ...previousState.omittedFields,
          ...data,
        },
      }
    case SET_COLUMNS_ORDER:
      return {
        ...previousState,
        columnsOrder: {
          ...previousState.columnsOrder,
          ...data,
        },
      }
    default:
      return previousState
  }
}
