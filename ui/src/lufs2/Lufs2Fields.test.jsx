import React from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { CurrentField } from './Lufs2Fields'

describe('Now column', () => {
  it('shows the latest measured audio instead of the original', () => {
    render(
      <CurrentField
        record={{
          loudnessAudit: {
            lufsBefore: -24,
            tpBefore: -9,
            lufsAfter: -12.8,
            tpAfter: -0.7,
          },
        }}
      />,
    )
    expect(screen.getByText('-12.80 LUFS')).toBeInTheDocument()
    expect(screen.getByText('peak -0.70 dBTP')).toBeInTheDocument()
    expect(screen.queryByText('-24.00 LUFS')).not.toBeInTheDocument()
  })

  it('says Not measured when readings are missing', () => {
    render(
      <CurrentField
        record={{ loudnessAudit: { lufsBefore: null, tpBefore: null } }}
      />,
    )
    expect(screen.getByText('Not measured')).toBeInTheDocument()
    expect(screen.getByText('peak: Not measured')).toBeInTheDocument()
    expect(screen.queryByText('0.00 LUFS')).not.toBeInTheDocument()
  })
})
