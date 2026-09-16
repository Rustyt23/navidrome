import React from 'react'
import JobProgress from './JobProgress'
import { StopLufsJobButton } from './LufsJobButtons'
import { RESTORE_URL, useRestoreStatus } from './useRestoreStatus'

// RestoreProgress shows a restore while it runs: how many of the songs have
// been done out of how many, what happened to them, and a button to stop.
//
// Both LUFS pages have a Restore button, and a restore started from either is
// the same background job, so both pages draw it from here. The exceptions page
// used to draw nothing at all, so a restore started there ran unseen.
export const RestoreProgress = () => {
  const { status } = useRestoreStatus()
  const detail = status?.running
    ? [
        `${status.restored || 0} restored`,
        `${status.skipped || 0} had no stored original`,
        `${status.failed || 0} failed`,
        status.cancelled ? `${status.cancelled} stopped` : null,
      ]
        .filter(Boolean)
        .join(' · ')
    : undefined

  return (
    <>
      <JobProgress label="Restoring songs" status={status} detail={detail} />
      {status?.running && (
        <StopLufsJobButton
          url={`${RESTORE_URL}/stop`}
          label="resources.lufs.actions.stopRestore"
          disabled={!!status.stopping}
        />
      )}
    </>
  )
}

export default RestoreProgress
