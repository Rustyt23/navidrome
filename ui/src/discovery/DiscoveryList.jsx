import React, { cloneElement, useCallback, useMemo, useState } from 'react'
import {
  Button,
  Datagrid,
  DateField,
  Filter,
  NumberField,
  SearchInput,
  TextField,
  TopToolbar,
  List,
  sanitizeListRestProps,
  useDataProvider,
  useNotify,
  useRefresh,
  useTranslate,
} from 'react-admin'
import { makeStyles, CircularProgress } from '@material-ui/core'
import RefreshIcon from '@material-ui/icons/Refresh'

const useActionStyles = makeStyles(
  (theme) => ({
    toolbar: {
      gap: theme.spacing(1),
    },
    spinner: {
      marginRight: theme.spacing(1),
    },
  }),
  { name: 'NDDiscoveryListActions' }
)

const DiscoveryFilter = (props) => (
  <Filter {...props} variant="outlined">
    <SearchInput source="q" alwaysOn />
  </Filter>
)

const DiscoveryListActions = ({ className, ...rest }) => {
  const classes = useActionStyles()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const refresh = useRefresh()
  const translate = useTranslate()
  const [loading, setLoading] = useState(false)

  const handleRefresh = useCallback(async () => {
    if (loading) return
    setLoading(true)
    try {
      await dataProvider.refreshDiscovery()
      refresh()
      notify('resources.discovery.messages.refreshed', 'info', {
        _: translate('resources.discovery.messages.refreshed'),
      })
    } catch (error) {
      notify('ra.page.error', 'warning')
    } finally {
      setLoading(false)
    }
  }, [dataProvider, loading, notify, refresh, translate])

  return (
    <TopToolbar
      className={`${className || ''} ${classes.toolbar}`.trim()}
      {...sanitizeListRestProps(rest)}
    >
      {rest.filters && cloneElement(rest.filters, { context: 'button' })}
      <Button
        label={translate('resources.discovery.actions.refresh')}
        onClick={handleRefresh}
        disabled={loading}
      >
        {loading ? <CircularProgress size={18} className={classes.spinner} /> : <RefreshIcon />}
      </Button>
    </TopToolbar>
  )
}

const rowClick = (id) => `/discovery/${id}`

const DiscoveryList = (props) => {
  const translate = useTranslate()
  const defaultSort = useMemo(() => ({ field: 'name', order: 'ASC' }), [])
  const title = useMemo(
    () => translate('resources.discovery.name', { smart_count: 2 }),
    [translate]
  )

  return (
    <List
      {...props}
      exporter={false}
      filters={<DiscoveryFilter />}
      actions={<DiscoveryListActions />}
      bulkActionButtons={false}
      perPage={50}
      sort={defaultSort}
      title={title}
    >
      <Datagrid rowClick={rowClick} optimized>
        <TextField source="name" />
        <NumberField
          source="songCount"
          label="resources.discovery.fields.songCount"
        />
        <DateField
          source="updatedAt"
          showTime
          label="resources.discovery.fields.updatedAt"
        />
      </Datagrid>
    </List>
  )
}

export default DiscoveryList
