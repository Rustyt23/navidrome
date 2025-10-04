import {
  Create,
  SimpleForm,
  TextInput,
  BooleanInput,
  required,
  useTranslate,
  useRefresh,
  useNotify,
  useRedirect,
  useResourceContext,
} from 'react-admin'
import { Title } from '../common'
import { useLocation } from 'react-router-dom'

const PlaylistCreate = (props) => {
  const refresh = useRefresh()
  const notify = useNotify()
  const redirect = useRedirect()
  const translate = useTranslate()
  const resource = useResourceContext() || 'playlist'
  const resourceName = translate(`resources.${resource}.name`, { smart_count: 1 })
  const title = translate('ra.page.create', {
    name: `${resourceName}`,
  })
  const location = useLocation()
  const folderStateKey = resource === 'discovery' ? 'discoveryFolderId' : 'playlistFolderId'
  const fallbackKey = resource === 'discovery' ? 'discoveryId' : 'playlistId'
  const folderId =
    location.state?.[folderStateKey] ??
    location.state?.folderId ??
    location.state?.[fallbackKey] ??
    null
  const folderBasePath = resource === 'discovery' ? '/discoveryFolder' : '/folder'

  const onSuccess = () => {
    notify('ra.notification.created', 'info', { smart_count: 1 })
    if (folderId) redirect(`${folderBasePath}/${folderId}/show`)
    else redirect('list', folderBasePath)
    refresh()
  }

  return (
    <Create title={<Title subTitle={title} />} {...props} onSuccess={onSuccess}>
      <SimpleForm redirect="list" variant={'outlined'}>
        <TextInput source="name" validate={required()} />
        <TextInput multiline source="comment" />
        <TextInput source="folderId" defaultValue={folderId} style={{ display: 'none' }} />
        <BooleanInput source="public" initialValue={true} />
      </SimpleForm>
    </Create>
  )
}

export default PlaylistCreate
