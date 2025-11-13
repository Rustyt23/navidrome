import polyglotI18nProvider from 'ra-i18n-polyglot'
import deepmerge from 'deepmerge'
import dataProvider from '../dataProvider'
import en from './en.json'
import { i18nProvider } from './index'

// Only returns current selected locale if its translations are found in localStorage
const defaultLocale = function () {
  const locale = localStorage.getItem('locale')
  const current = JSON.parse(localStorage.getItem('translation'))
  if (current && current.id === locale) {
    // Asynchronously reload the translation from the server
    retrieveTranslation(locale).then(() => {
      i18nProvider.changeLocale(locale)
    })
    return locale
  }
  return 'en'
}

export function retrieveTranslation(locale) {
  return dataProvider.getOne('translation', { id: locale }).then((res) => {
    localStorage.setItem('translation', JSON.stringify(res.data))
    return prepareLanguage(JSON.parse(res.data.data))
  })
}

const removeEmpty = (obj) => {
  for (let k in obj) {
    if (
      Object.prototype.hasOwnProperty.call(obj, k) &&
      typeof obj[k] === 'object'
    ) {
      removeEmpty(obj[k])
    } else {
      if (!obj[k]) {
        delete obj[k]
      }
    }
  }
}

const mergeResources = (baseResource, overrideResource) => {
  if (!baseResource) {
    return overrideResource
  }

  if (!overrideResource) {
    return baseResource
  }

  return deepmerge(baseResource, overrideResource, {
    arrayMerge: (_destinationArray, sourceArray) => sourceArray,
  })
}

const prepareLanguage = (lang) => {
  removeEmpty(lang)
  // Make "albumSong", "playlistTrack" and "discoveryTrack" resources inherit translations from "song"
  lang.resources.albumSong = mergeResources(
    lang.resources.song,
    lang.resources.albumSong,
  )
  lang.resources.playlistTrack = mergeResources(
    lang.resources.song,
    lang.resources.playlistTrack,
  )
  lang.resources.discoveryTrack = mergeResources(
    lang.resources.song,
    lang.resources.discoveryTrack,
  )
  if (lang.resources.playlist) {
    lang.resources.discovery = lang.resources.playlist
  }
  // ra.boolean.null should always be empty
  lang.ra.boolean.null = ''
  // Fallback to english translations
  return deepmerge(en, lang)
}

export default polyglotI18nProvider((locale) => {
  // English is bundled
  if (locale === 'en') {
    return prepareLanguage(en)
  }
  // If the requested locale is in already loaded, return it
  const current = JSON.parse(localStorage.getItem('translation'))
  if (current && current.id === locale) {
    return prepareLanguage(JSON.parse(current.data))
  }
  // If not, get it from the server, and store it in localStorage
  return retrieveTranslation(locale)
}, defaultLocale())
