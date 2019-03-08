// Copyright 2018 github.com/ucirello
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package mail handles the email based link intake.
package mail

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"strings"
	"time"

	"cirello.io/bookmarkd/pkg/actions"
	"cirello.io/bookmarkd/pkg/models"
	"cirello.io/bookmarkd/pkg/pubsub"
	smtp "github.com/emersion/go-smtp"
	"github.com/jmoiron/sqlx"
	xurls "mvdan.cc/xurls/v2"
)

type backend struct {
	db        *sqlx.DB
	sender    string
	recipient string
	pubsub    *pubsub.Broker
}

func (b *backend) Login(username, password string) (smtp.User, error) {
	return nil, errors.New("bad credentials")
}

func (b *backend) AnonymousLogin() (smtp.User, error) {
	return &user{backend: b}, nil
}

type user struct {
	backend *backend
}

func (u *user) Send(from string, to []string, r io.Reader) error {
	if !u.isValidOrigin(from, to) {
		return nil
	}
	u.extractLink(r)
	return nil
}

func (u *user) Logout() error {
	return nil
}

func (u *user) isValidOrigin(from string, to []string) bool {
	if from != u.backend.sender {
		log.Println("acceptable sender not found:", from)
		return false
	}
	if len(to) != 1 && to[0] != u.backend.recipient {
		log.Println("bad recipient", to)
		return false
	}
	return true
}

func (u *user) extractLink(r io.Reader) {
	m, err := mail.ReadMessage(r)
	if err != nil {
		log.Println("cannot read email:", err)
		return
	}
	go func() {
		log.Println("extractLink start")
		defer log.Println("extractLink done")
		mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
		if err != nil {
			log.Println("cannot load content-type:", err)
			return
		}
		if !strings.HasPrefix(mediaType, "multipart/") {
			log.Println("not a multipart message")
			return
		}
		mr := multipart.NewReader(m.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				return
			} else if err != nil {
				log.Println("cannot parse part of the message:", err)
				return
			}
			if contentType := p.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
				log.Println("skipped part of the message:", contentType)
				continue
			}
			body, err := ioutil.ReadAll(p)
			if err != nil {
				log.Println("cannot read body of the part of the message:", err)
				return
			}
			urls := xurls.Strict().FindAllString(string(body), -1)
			if len(urls) == 0 {
				log.Println("cannot find link:", err)
				return
			}
			dec := new(mime.WordDecoder)
			title, err := dec.DecodeHeader(m.Header.Get("subject"))
			if err != nil {
				log.Println("cannot parse email subject:", err)
				return
			}
			err = actions.AddBookmark(u.backend.db, &models.Bookmark{
				URL:             urls[0],
				Title:           title,
				Inbox:           1,
				LastStatusCode:  200,
				LastStatusCheck: time.Now().Unix(),
			}, u.backend.pubsub.Broadcast)
			if err != nil {
				log.Println("cannot store new link:", err)
			}
			log.Println("added:", urls[0])
		}
	}()
}

// Run serves the MX service for email-based link intake.
func Run(l net.Listener, db *sqlx.DB, broker *pubsub.Broker, domain, sender, recipient string) {
	be := &backend{
		db:        db,
		sender:    sender,
		recipient: recipient,
		pubsub:    broker,
	}
	s := smtp.NewServer(be)
	s.Addr = l.Addr().String()
	s.Domain = domain
	s.MaxIdleSeconds = 300
	s.MaxMessageBytes = 2 * 1024 * 1024
	s.MaxRecipients = 1
	s.AllowInsecureAuth = true
	go func() {
		if err := s.Serve(l); err != nil {
			log.Fatal(err)
		}
	}()
}
