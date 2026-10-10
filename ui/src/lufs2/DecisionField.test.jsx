import React from 'react'
import { cleanup, render, screen } from '@testing-library/react'
import { RecordContextProvider } from 'react-admin'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { DecisionField } from './Lufs2Fields'

const labels = {
  'resources.lufs2.decision.pending': 'Undecided',
  'resources.lufs2.decision.gain_ceiling': 'Gain to ceiling',
  'resources.lufs2.decision.skip': 'Leave alone',
  'resources.lufs2.decision.limit': 'Limit to target',
}

vi.mock('react-admin', async (importOriginal) => ({
  ...(await importOriginal()),
  useTranslate: () => (key, options) => labels[key] || options?._ || key,
}))

afterEach(cleanup)

const result = (audit = {}) => (
  <RecordContextProvider
    value={{
      id: 'song-1',
      loudnessAudit: {
        status: 'analyzed',
        lufsBefore: -11.68,
        tpBefore: -0.37,
        ...audit,
      },
    }}
  >
    <DecisionField />
  </RecordContextProvider>
)

describe('Saved decision labels', () => {
  it.each([
    ['gain_ceiling', 'Gain to ceiling'],
    ['skip', 'Leave alone'],
  ])('keeps %s visible after reloading the record', (decision, label) => {
    render(result({ decision })).unmount()
    render(result({ decision }))

    expect(screen.getByText(label)).toBeInTheDocument()
    expect(screen.queryByText('Undecided')).not.toBeInTheDocument()
  })

  it('keeps Gain to ceiling alongside a measured result', () => {
    render(
      result({
        decision: 'gain_ceiling',
        status: 'processed',
        action: 'gained',
        lufsAfter: -12.6,
        tpAfter: -0.5,
      }),
    )

    expect(screen.getByText('Gain to ceiling')).toBeInTheDocument()
    expect(screen.getByText('-12.60 LUFS')).toBeInTheDocument()
  })

  it('keeps Gain to ceiling when an attempted rewrite was refused', () => {
    render(
      result({
        decision: 'gain_ceiling',
        action: 'refused',
        error: 'output was not format-identical: [duration changed]',
      }),
    )

    expect(screen.getByText('Gain to ceiling')).toBeInTheDocument()
    expect(screen.queryByText('Undecided')).not.toBeInTheDocument()
  })

  it('replaces the saved label when the decision changes', () => {
    const view = render(result({ decision: 'gain_ceiling' }))
    view.rerender(result({ decision: 'skip' }))

    expect(screen.getByText('Leave alone')).toBeInTheDocument()
    expect(screen.queryByText('Gain to ceiling')).not.toBeInTheDocument()

    view.rerender(result({ decision: 'limit' }))

    expect(screen.getByText('Limit to target')).toBeInTheDocument()
    expect(screen.queryByText('Leave alone')).not.toBeInTheDocument()
  })

  it('shows Undecided when no decision is saved', () => {
    render(result())

    expect(screen.getByText('Undecided')).toBeInTheDocument()
    expect(screen.getByText('nothing applied yet')).toBeInTheDocument()
  })
})
