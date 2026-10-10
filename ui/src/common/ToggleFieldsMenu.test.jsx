import React from 'react'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react'
import { Provider } from 'react-redux'
import { combineReducers, createStore } from 'redux'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ToggleFieldsMenu } from './ToggleFieldsMenu'
import { settingsReducer } from '../reducers/settingsReducer'
import { SET_COLUMNS_ORDER, SET_TOGGLEABLE_FIELDS } from '../actions/settings'
import english from '../i18n/en.json'

const mocks = vi.hoisted(() => ({ dragContext: null }))

vi.mock('react-admin', async (importOriginal) => ({
  ...(await importOriginal()),
  useTranslate: () => (key) =>
    key.split('.').reduce((value, part) => value?.[part], english) || key,
}))

vi.mock('@dnd-kit/core', () => ({
  DndContext: (props) => {
    mocks.dragContext = props
    return props.children
  },
  DragOverlay: (props) => (
    <div data-testid="column-drag-overlay">{props.children}</div>
  ),
  useDraggable: () => ({ attributes: {}, listeners: {}, setNodeRef: vi.fn() }),
  useDroppable: () => ({ setNodeRef: vi.fn() }),
}))

afterEach(cleanup)

const openMenu = (props = { resource: 'lufs2', preferenceKey: 'lufs2.v2' }) => {
  const store = createStore(combineReducers({ settings: settingsReducer }), {
    settings: {
      toggleableFields: {
        'lufs2.v2': { artist: true, decision: true, optionCeiling: false },
        song: { title: true, duration: false },
      },
      omittedFields: { 'lufs2.v2': [], song: [] },
      columnsOrder: {
        'lufs2.v2': ['artist', 'decision', 'optionCeiling'],
        song: ['title', 'duration'],
      },
    },
  })
  const dispatch = vi.spyOn(store, 'dispatch')
  render(
    <Provider store={store}>
      <ToggleFieldsMenu {...props} />
    </Provider>,
  )
  fireEvent.click(screen.getByRole('button', { name: 'more' }))
  return { store, dispatch }
}

describe('Column picker resource and saved layout', () => {
  it('translates labels using the resource while reading versioned preferences', () => {
    openMenu()

    for (const key of ['artist', 'decision', 'optionCeiling']) {
      expect(
        screen.getByRole('menuitem', {
          name: english.resources.lufs2.fields[key],
        }),
      ).toBeInTheDocument()
    }
    expect(screen.queryByText(/resources\.lufs2\.v2/)).not.toBeInTheDocument()
    expect(
      within(screen.getByRole('menuitem', { name: 'Artist' })).getByRole(
        'checkbox',
      ),
    ).toBeChecked()
    expect(
      within(
        screen.getByRole('menuitem', {
          name: english.resources.lufs2.fields.optionCeiling,
        }),
      ).getByRole('checkbox'),
    ).not.toBeChecked()
  })

  it('writes visibility to the versioned layout and preserves other selections', () => {
    const { store, dispatch } = openMenu()
    fireEvent.click(
      screen.getByRole('menuitem', {
        name: english.resources.lufs2.fields.optionCeiling,
      }),
    )

    expect(dispatch).toHaveBeenCalledWith({
      type: SET_TOGGLEABLE_FIELDS,
      data: {
        'lufs2.v2': { artist: true, decision: true, optionCeiling: true },
      },
    })
    expect(store.getState().settings.toggleableFields).toEqual({
      'lufs2.v2': { artist: true, decision: true, optionCeiling: true },
      song: { title: true, duration: false },
    })
  })

  it('continues to use the resource as the default preference key', () => {
    const { store, dispatch } = openMenu({ resource: 'song' })
    fireEvent.click(
      screen.getByRole('menuitem', {
        name: english.resources.song.fields.duration,
      }),
    )

    expect(dispatch).toHaveBeenCalledWith({
      type: SET_TOGGLEABLE_FIELDS,
      data: { song: { title: true, duration: true } },
    })
    expect(store.getState().settings.toggleableFields['lufs2.v2']).toEqual({
      artist: true,
      decision: true,
      optionCeiling: false,
    })
  })

  it('uses the translated resource label in the drag overlay', () => {
    openMenu()
    act(() => mocks.dragContext.onDragStart({ active: { id: 'decision' } }))

    const overlay = within(screen.getByTestId('column-drag-overlay'))
    expect(
      overlay.getByText(english.resources.lufs2.fields.decision),
    ).toBeInTheDocument()
    expect(overlay.queryByText(/resources\.lufs2\.v2/)).not.toBeInTheDocument()
  })

  it('keeps reordered columns under the versioned preference key', () => {
    const { store, dispatch } = openMenu()
    act(() =>
      mocks.dragContext.onDragEnd({
        active: { id: 'artist' },
        over: { id: 'decision' },
      }),
    )

    expect(dispatch).toHaveBeenCalledWith({
      type: SET_COLUMNS_ORDER,
      data: { 'lufs2.v2': ['decision', 'artist', 'optionCeiling'] },
    })
    expect(store.getState().settings.columnsOrder).toEqual({
      'lufs2.v2': ['decision', 'artist', 'optionCeiling'],
      song: ['title', 'duration'],
    })
  })
})
