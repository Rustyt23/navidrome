import React from 'react'
import { Provider } from 'react-redux'
import { combineReducers, createStore } from 'redux'
import { renderHook, act } from '@testing-library/react-hooks'
import { waitFor } from '@testing-library/react'
import useThemePreferenceSync from './useThemePreferenceSync'
import { themeReducer } from '../reducers/themeReducer'
import httpClient from '../dataProvider/httpClient'
import { changeTheme } from '../actions'

const createThemeStore = (theme) =>
  createStore(
    combineReducers({ theme: themeReducer }),
    theme ? { theme } : undefined,
  )

vi.mock('../dataProvider/httpClient', () => ({
  default: vi.fn(),
}))

describe('useThemePreferenceSync', () => {
  beforeEach(() => {
    httpClient.mockReset()
  })

  it('applies the stored theme from the server', async () => {
    httpClient.mockResolvedValueOnce({
      json: { theme: 'SpotifyTheme', isDefault: false },
    })

    const store = createThemeStore()
    const wrapper = ({ children }) => (
      <Provider store={store}>{children}</Provider>
    )

    renderHook(() => useThemePreferenceSync(), { wrapper })

    await waitFor(() => expect(store.getState().theme).toBe('SpotifyTheme'))
    expect(httpClient).toHaveBeenCalledTimes(1)
  })

  it('persists an existing local theme when the server has no preference', async () => {
    httpClient
      .mockResolvedValueOnce({
        json: { theme: 'Music Matters', isDefault: true },
      })
      .mockResolvedValue({ json: {} })

    const store = createThemeStore('DarkTheme')
    const wrapper = ({ children }) => (
      <Provider store={store}>{children}</Provider>
    )

    renderHook(() => useThemePreferenceSync(), { wrapper })

    await waitFor(() => expect(httpClient).toHaveBeenCalledTimes(2))
    expect(store.getState().theme).toBe('DarkTheme')
    expect(JSON.parse(httpClient.mock.calls[1][1].body)).toEqual({
      theme: 'DarkTheme',
    })
  })

  it('saves theme changes after the initial synchronisation', async () => {
    httpClient
      .mockResolvedValueOnce({
        json: { theme: 'Music Matters', isDefault: true },
      })
      .mockResolvedValue({ json: {} })

    const store = createThemeStore('MusicMattersTheme')
    const wrapper = ({ children }) => (
      <Provider store={store}>{children}</Provider>
    )

    renderHook(() => useThemePreferenceSync(), { wrapper })

    await waitFor(() => expect(httpClient).toHaveBeenCalledTimes(1))

    await act(async () => {
      store.dispatch(changeTheme('GreenTheme'))
    })

    await waitFor(() => expect(httpClient).toHaveBeenCalledTimes(2))
    expect(JSON.parse(httpClient.mock.calls[1][1].body)).toEqual({
      theme: 'GreenTheme',
    })
  })
})

