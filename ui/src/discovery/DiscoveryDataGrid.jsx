import React, { isValidElement } from 'react'
import { Datagrid, PureDatagridBody, PureDatagridRow } from 'react-admin'
import clsx from 'clsx'
import { makeStyles } from '@material-ui/core/styles'
import { useHistory } from 'react-router-dom'

const useStyles = makeStyles({
  row: {
    cursor: 'pointer',
    '&:hover': { backgroundColor: '#f5f5f5' },
    '& td': { paddingTop: 3, paddingBottom: 3 },
  },
  missingRow: {
    cursor: 'inherit',
    opacity: 0.3,
  },
  headerStyle: {
    '& thead': { boxShadow: '0px 3px 3px rgba(0,0,0,.15)' },
    '& th': { fontWeight: 'bold', padding: '15px' },
  },
})

const DiscoveryRow = ({ record, children, className, rowClick, ...rest }) => {
  const classes = useStyles()
  const history = useHistory()
  const fields = React.Children.toArray(children).filter((c) => isValidElement(c))

  const computedClasses = clsx(className, classes.row, record?.missing && classes.missingRow)

  const handleRowClick = (event) => {
    if (typeof rowClick !== 'function') return
    event.preventDefault()
    const target = rowClick(record?.id, record)
    if (target) history.push(target)
  }

  return (
    <PureDatagridRow
      record={record}
      {...rest}
      className={computedClasses}
      onClick={handleRowClick}
    >
      {fields}
    </PureDatagridRow>
  )
}

const DiscoveryDatagridBody = (props) => <PureDatagridBody {...props} row={<DiscoveryRow />} />

const DiscoveryDataGrid = (props) => {
  const classes = useStyles()
  return (
    <Datagrid
      className={classes.headerStyle}
      isRowSelectable={() => false}
      body={<DiscoveryDatagridBody />}
      {...props}
    />
  )
}

export default DiscoveryDataGrid
