export const SET_NOTIFICATIONS_STATE = 'SET_NOTIFICATIONS_STATE'
export const SET_TOGGLEABLE_FIELDS = 'SET_TOGGLEABLE_FIELDS'
export const SET_OMITTED_FIELDS = 'SET_OMITTED_FIELDS'
export const SET_COLUMNS_ORDER = 'SET_COLUMNS_ORDER'
export const SET_APPBAR_ICONS = 'SET_APPBAR_ICONS'
export const SET_MUSICBRAINZ_VISIBLE = 'SET_MUSICBRAINZ_VISIBLE'
export const SET_NAMING_ENFORCEMENT = 'SET_NAMING_ENFORCEMENT'

export const setNotificationsState = (enabled) => ({
  type: SET_NOTIFICATIONS_STATE,
  data: enabled,
})

export const setToggleableFields = (obj) => ({
  type: SET_TOGGLEABLE_FIELDS,
  data: obj,
})

export const setOmittedFields = (obj) => ({
  type: SET_OMITTED_FIELDS,
  data: obj,
})

export const setColumnsOrder = (obj) => ({
  type: SET_COLUMNS_ORDER,
  data: obj,
})

export const setAppBarIcons = (obj) => ({
  type: SET_APPBAR_ICONS,
  data: obj,
})

export const setMusicBrainzVisible = (visible) => ({
  type: SET_MUSICBRAINZ_VISIBLE,
  data: visible,
})

// Mirrors the server-side naming gate. Unlike the other settings here this one
// is not a client preference: it is persisted on the server, because the server
// is what actually blocks the upload. The store only caches it so the toggle
// and the notification icon agree without each refetching.
export const setNamingEnforcement = (enforced) => ({
  type: SET_NAMING_ENFORCEMENT,
  data: enforced,
})
