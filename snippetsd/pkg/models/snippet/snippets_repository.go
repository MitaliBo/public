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

package snippet

import "cirello.io/snippetsd/pkg/models/user"

// Repository provides a repository of Snippets.
type Repository interface {
	Bootstrap() error
	All() ([]*Snippet, error)
	GetByUser(*user.User) ([]*Snippet, error)
	Current() ([]*Snippet, error)
	Insert(snippet *Snippet) (*Snippet, error)
	Update(snippet *Snippet) error
}
