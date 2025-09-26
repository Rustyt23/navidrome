import React from 'react'
import { Route } from 'react-router-dom'
import Personal from './personal/Personal'
import { ReorderableSongList } from './song'

const routes = [
  <Route exact path="/personal" render={() => <Personal />} key={'personal'} />,
  <Route
    exact
    path="/reorder-songs"
    render={() => <ReorderableSongList />}
    key={'reorder-songs'}
  />,
]

export default routes
