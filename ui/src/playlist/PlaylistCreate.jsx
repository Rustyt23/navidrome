import {
  Create,
  SimpleForm,
  TextInput,
  BooleanInput,
  required,
  useTranslate,
  useRefresh,
  useNotify,
  useRedirect
} from 'react-admin'
import { Title } from '../common'
import { useLocation } from 'react-router-dom'

const PlaylistCreate = (props) => {
  const refresh = useRefresh()
  const notify = useNotify()
  const redirect = useRedirect()
  const translate = useTranslate()
  const resourceName = translate('resources.playlist.name', { smart_count: 1 })
  const title = translate('ra.page.create', {
    name: `${resourceName}`,
  })
  const location = useLocation()
  const playlistFolderId = location.state?.playlistFolderId || null

  const onSuccess = () => {
    notify('ra.notification.created', 'info', { smart_count: 1 })
    if (playlistFolderId) redirect(`/folder/${playlistFolderId}/show`)
    else redirect('list', '/folder')
    refresh()
  }

  return (
    <Create title={<Title subTitle={title} />} {...props} onSuccess={onSuccess}>
      <SimpleForm redirect="list" variant={'outlined'}>
        <TextInput source="name" validate={required()} />
        <TextInput multiline source="comment" />
        <TextInput source="folderId" defaultValue={playlistFolderId} style={{ display: 'none' }} />
        <BooleanInput source="public" initialValue={true} />
      </SimpleForm>
    </Create>
  )
}

export default PlaylistCreate
