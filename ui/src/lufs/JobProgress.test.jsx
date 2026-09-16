import React from 'react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { JobProgress } from './JobProgress'

describe('finished LUFS job', () => {
  it('keeps rejected results visible after processing finishes', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{ running: false, rejected: 2 }}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      '2 rejected; working audio unchanged',
    )
  })

  it('reports both record-save errors and rejected candidates', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{
          running: false,
          rejected: 1,
          error: 'Could not save LUFS records',
        }}
      />,
    )
    expect(screen.getAllByRole('alert')).toHaveLength(2)
    expect(screen.getByText('Could not save LUFS records')).toBeInTheDocument()
  })
})

describe('job error in the red box', () => {
  // What a song stopped part way used to put on the page.
  const wallOfText =
    "verifying result: analyzing loudness: signal: killed: Input #0, mp3, from '/Users/vikashsingh/Documents/work/music-storage/music/.Chill & Groove - Haramayare.mp3.lufs-773454236.mp3': " +
    'size=N/A time=00:00:03.30 bitrate=N/A speed=6.52x elapsed=0:00:00.50 '.repeat(
      200,
    )

  it('shows about one line, with the full text on hover', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{ running: false, error: wallOfText }}
      />,
    )
    const alert = screen.getByRole('alert')
    expect(alert.textContent.length).toBeLessThanOrEqual(161)
    expect(alert.textContent).toMatch(
      /^verifying result: analyzing loudness: signal: killed/,
    )
    expect(alert.textContent.endsWith('…')).toBe(true)
    expect(alert).toHaveAttribute('title', wallOfText)
  })

  it('leaves a short error exactly as it is', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{
          running: true,
          processed: 1,
          total: 2,
          error: 'probing "song.mp3": exit status 1',
        }}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(
      /^probing "song.mp3": exit status 1$/,
    )
  })

  it('shows only the first of several joined errors', () => {
    render(
      <JobProgress
        label="Optimising"
        status={{
          running: false,
          error: 'could not read file\ncould not save LUFS records',
        }}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent(/^could not read file$/)
  })
})
