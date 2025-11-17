import React from 'react'
import { Container, Typography } from '@material-ui/core'

const MetadataPage = () => (
  <Container maxWidth="lg" style={{ padding: '2rem 1.5rem' }}>
    <Typography variant="h4" component="h1" gutterBottom>
      MetaData
    </Typography>
    <Typography variant="subtitle1" color="textSecondary">
      Manage and enhance song metadata
    </Typography>
  </Container>
)

export default MetadataPage
