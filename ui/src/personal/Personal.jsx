import { SimpleForm, Title, usePermissions, useTranslate } from 'react-admin'
import { Card } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { SelectLanguage } from './SelectLanguage'
import { SelectTheme } from './SelectTheme'
import { SelectDefaultView } from './SelectDefaultView'
import { NotificationsToggle } from './NotificationsToggle'
import { LastfmScrobbleToggle } from './LastfmScrobbleToggle'
import { ListenBrainzScrobbleToggle } from './ListenBrainzScrobbleToggle'
import config from '../config'
import { ReplayGainToggle } from './ReplayGainToggle'
import {
  NowPlayingIconToggle,
  MissingTracksIconToggle,
} from './AppBarIconToggles'
import { ProcessLufsButton } from './ProcessLufsButton'

const useStyles = makeStyles({
  root: { marginTop: '1em' },
})

const Personal = () => {
  const translate = useTranslate()
  const classes = useStyles()
  const { permissions } = usePermissions()

  return (
    <Card className={classes.root}>
      <Title title={'MusicMatters - ' + translate('menu.personal.name')} />
      <SimpleForm toolbar={null} variant={'outlined'}>
        <SelectTheme />
        <SelectLanguage />
        <SelectDefaultView />
        {config.enableReplayGain && <ReplayGainToggle />}
        <NotificationsToggle />
        {config.enableNowPlaying && permissions === 'admin' && (
          <NowPlayingIconToggle />
        )}
        {permissions === 'admin' && <MissingTracksIconToggle />}
        {permissions === 'admin' && <ProcessLufsButton />}
        {config.lastFMEnabled && <LastfmScrobbleToggle />}
        {config.listenBrainzEnabled && <ListenBrainzScrobbleToggle />}
      </SimpleForm>
    </Card>
  )
}

export default Personal
