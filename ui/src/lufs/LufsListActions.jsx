import React from 'react'
import { ExportButton, TopToolbar, useListContext } from 'react-admin'
import { ToggleFieldsMenu } from '../common'
import LufsToggle from './LufsToggle'
import { AnalyzeLufsButton } from './LufsAnalyzeButton'
import SpeedIcon from '@material-ui/icons/Speed'
import JobProgress from './JobProgress'
import { ClearLufsAnalysisButton, StopLufsJobButton } from './LufsJobButtons'
import { ANALYZE_URL } from './useAnalyzeStatus'
import { LIBRARY_URL } from './useLibraryStatus'

const LufsListActions = ({
  onSettingsChange,
  analyzeStatus,
  onAnalyzeStarted,
  libraryStatus,
  onOptimiseAllToggled,
  ...rest
}) => {
  const { total } = useListContext()

  const optimiseDetail = libraryStatus?.running
    ? `${libraryStatus.normalized || 0} changed · ${libraryStatus.skipped || 0} already fine · ${libraryStatus.failed || 0} failed`
    : undefined
  const analyzeDetail = analyzeStatus?.running
    ? `${analyzeStatus.failed || 0} failed`
    : undefined

  return (
    <TopToolbar {...rest}>
      <JobProgress
        label="Optimising"
        status={libraryStatus}
        detail={optimiseDetail}
      />
      <JobProgress
        label="Analysing"
        status={analyzeStatus}
        detail={analyzeDetail}
      />
      {libraryStatus?.running && (
        <StopLufsJobButton
          url={`${LIBRARY_URL}/stop`}
          label="resources.lufs.actions.stopOptimisation"
          disabled={libraryStatus?.stopping}
          onStopped={onOptimiseAllToggled}
        />
      )}
      {analyzeStatus?.running && (
        <StopLufsJobButton
          url={`${ANALYZE_URL}/stop`}
          label="resources.lufs.actions.stopAnalysis"
          disabled={analyzeStatus?.stopping}
          onStopped={onAnalyzeStarted}
        />
      )}
      <LufsToggle
        onChange={onSettingsChange}
        onToggled={onOptimiseAllToggled}
        disabled={!!analyzeStatus?.running}
      />
      <AnalyzeLufsButton
        all
        mode="original"
        label="resources.lufs.actions.fetchOriginal"
        icon={<SpeedIcon />}
        onStarted={onAnalyzeStarted}
        disabled={!!analyzeStatus?.running || !!libraryStatus?.running}
      />
      <AnalyzeLufsButton
        all
        onStarted={onAnalyzeStarted}
        disabled={!!analyzeStatus?.running || !!libraryStatus?.running}
      />
      <ClearLufsAnalysisButton
        disabled={!!analyzeStatus?.running || !!libraryStatus?.running}
      />
      <ExportButton maxResults={total} />
      <ToggleFieldsMenu resource="lufs" />
    </TopToolbar>
  )
}

export default LufsListActions
