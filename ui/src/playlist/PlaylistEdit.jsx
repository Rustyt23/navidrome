import { useCallback } from 'react'
import {
  Edit,
  FormDataConsumer,
  SimpleForm,
  TextInput,
  TextField,
  BooleanInput,
  required,
  useTranslate,
  usePermissions,
  ReferenceInput,
  SelectInput,
  useNotify,
  useRedirect,
  useRefresh,
  useDataProvider,
  useRecordContext,
  Toolbar,
  SaveButton,
} from 'react-admin'
import { useLocation } from 'react-router-dom'
import { useFormState } from 'react-final-form'
import { makeStyles } from '@material-ui/core/styles'
import Button from '@material-ui/core/Button'
import DeleteIcon from '@material-ui/icons/Delete'
import { isWritable, Title } from '../common'

const SyncFragment = ({ formData, variant, ...rest }) => {
  return (
  <>
    {formData.path && <BooleanInput source="sync" {...rest} />}
    {formData.path && <TextField source="path" {...rest} />}
  </>
)
}

const PlaylistTitle = ({ record }) => {
  const translate = useTranslate()
  const resourceName = translate('resources.playlist.name', { smart_count: 1 })
  return <Title subTitle={`${resourceName} "${record ? record.name : ''}"`} />
}

const useToolbarStyles = makeStyles((theme) => ({
  root: { display: 'flex', alignItems: 'center', padding: theme.spacing(1, 2) },
  grow: { flex: 1 },
  delete: { color: theme.palette.error.main },
}))

const PlaylistEditToolbar = ({ handleSubmitWithRedirect, saving }) => {
  const classes = useToolbarStyles()
  const record = useRecordContext()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const redirect = useRedirect()
  const refresh = useRefresh()
  const location = useLocation()

  const { pristine, submitting, invalid } = useFormState({
    subscription: { pristine: true, submitting: true, invalid: true },
  })
  const saveDisabled = pristine || submitting || invalid

  const handleDelete = async () => {
    if (!record?.id) return
    try {
      await dataProvider.delete('playlist', { id: record.id })
      notify('ra.notification.deleted', { type: 'info', messageArgs: { smart_count: 1 } })

      const folderId = record?.folderId ?? location.state?.folderId ?? null
      if (folderId) redirect(`/folder/${folderId}/show`)
      else redirect('list', '/folder')

      refresh()
    } catch (e) {
      notify(e?.message || 'ra.notification.http_error', { type: 'warning' })
    }
  }

  return (
    <Toolbar className={classes.root}>
      <SaveButton
        variant="contained"
        color="primary"
        disabled={saveDisabled}
        saving={saving ?? submitting}
        handleSubmitWithRedirect={handleSubmitWithRedirect}
      />
      <span className={classes.grow} />
      <Button onClick={handleDelete} startIcon={<DeleteIcon />} className={classes.delete}>
        DELETE
      </Button>
    </Toolbar>
  )
}

const PlaylistEditForm = () => {
  const { permissions } = usePermissions()
  const record = useRecordContext()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const redirect = useRedirect()
  const refresh = useRefresh()
  const location = useLocation()

  const savePlaylist = useCallback(
    async (values) => {
      if (!record?.id) return
      try {
        const res = await dataProvider.update('playlist', { id: record.id, data: values })
        const saved = res?.data ?? values
        notify('ra.notification.updated', { type: 'info', messageArgs: { smart_count: 1 } })

        const folderId =
          saved?.folderId ?? saved?.folder_id ?? location.state?.folderId ?? null

        if (folderId) redirect(`/folder/${folderId}/show`)
        else redirect('list', '/folder')

        refresh()
      } catch (e) {
        notify(e?.message || 'ra.page.error', { type: 'warning' })
      }
    },
    [dataProvider, record?.id, notify, redirect, refresh, location]
  )

  return (
    <SimpleForm
      save={savePlaylist}
      variant={'outlined'}
      redirect={false}
      toolbar={<PlaylistEditToolbar />}
    >
      <TextInput source="name" validate={required()} />
      <TextInput multiline source="comment" />
      {permissions === 'admin' ? (
        <ReferenceInput
          source="ownerId"
          reference="user"
          perPage={0}
          sort={{ field: 'name', order: 'ASC' }}
        >
          <SelectInput
            label={'resources.playlist.fields.ownerName'}
            optionText="userName"
          />
        </ReferenceInput>
      ) : (
        <TextField source="ownerName" />
      )}

      <FormDataConsumer>
        {({ formData }) => (
          <BooleanInput source="public" disabled={!isWritable(formData?.ownerId)} />
        )}
      </FormDataConsumer>

      <FormDataConsumer>
        {(formDataProps) => <SyncFragment {...formDataProps} />}
      </FormDataConsumer>
    </SimpleForm>
  )
}

const PlaylistEdit = (props) => (
  <Edit title={<PlaylistTitle />} actions={false} {...props}>
    <PlaylistEditForm />
  </Edit>
)

export default PlaylistEdit
