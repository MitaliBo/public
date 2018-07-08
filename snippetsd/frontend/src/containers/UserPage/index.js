// Copyright 2018 github.com/ucirello
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React from 'react'
import { bindActionCreators } from 'redux'
import { connect } from 'react-redux'
import {
  Button,
  Col,
  Form,
  Grid,
  PageHeader,
  Row,
  FormControl
} from 'react-bootstrap'
import { loadSnippetsByUser } from './actions'
import groupBy from 'lodash/groupBy'
import moment from 'moment'

import './style.css'

class SubmitSnippetPage extends React.Component {
  componentDidMount () {
    this.props.loadSnippetsByUser()
  }

  render () {
    var snippets = groupBy(this.props.snippets, function (v) {
      return v.week_start
    })
    console.log(snippets)
    return (
      <Grid>
        <Row>
          <Col>
            <PageHeader> What did you do past week? </PageHeader>

            <Form>
              <FormControl componentClass='textarea' className='user-snippet-content' />
              <div className='user-snippet-submit'><Button>submit</Button></div>
            </Form>
          </Col>
        </Row>
        <Row>
          <Col>
            <PageHeader> Past Snippets </PageHeader>
            {
              Object.entries(snippets).map((week) => (
                <div key={week[0]} className='user-past-snippet'>
                  <strong>Week starting {moment(week[0]).format('MMMM Do YYYY')}: </strong>
                  { week[1].map((snippet) => (<div key={snippet.user.email}>{snippet.contents}</div>)) }
                </div>
              ))
            }
          </Col>
        </Row>
      </Grid>
    )
  }
}

const s2p = state => ({ snippets: state.snippets.snippets })
const d2p = dispatch => bindActionCreators({
  loadSnippetsByUser
}, dispatch)
export default connect(s2p, d2p)(SubmitSnippetPage)
