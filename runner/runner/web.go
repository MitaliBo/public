// Copyright 2017 github.com/ucirello
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

package runner

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"

	oversight "cirello.io/oversight/easy"
	"github.com/gorilla/websocket"
)

func (r *Runner) serveWeb(ctx context.Context) error {
	addr := r.ServiceDiscoveryAddr
	if addr == "" {
		return nil
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	log.Println("starting service discovery on", l.Addr())
	r.ServiceDiscoveryAddr = l.Addr().String()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			enc := json.NewEncoder(w)
			enc.SetIndent("", "    ")
			r.sdMu.Lock()
			defer r.sdMu.Unlock()
			err := enc.Encode(r.dynamicServiceDiscovery)
			if err != nil {
				log.Println("cannot serve service discovery request:", err)
			}
		})
		mux.HandleFunc("/logs", func(w http.ResponseWriter, req *http.Request) {
			filter := req.URL.Query().Get("filter")
			r.logsMu.Lock()
			stream := make(chan LogMessage, websocketLogForwarderBufferSize)
			r.logSubscribers = append(r.logSubscribers, stream)
			r.logsMu.Unlock()
			defer func() {
				r.logsMu.Lock()
				defer r.logsMu.Unlock()
				for i := 0; i < len(r.logSubscribers); i++ {
					if r.logSubscribers[i] == stream {
						r.logSubscribers = append(r.logSubscribers[:i], r.logSubscribers[i+1:]...)
						return
					}
				}
			}()
			upgrader := websocket.Upgrader{}
			c, err := upgrader.Upgrade(w, req, nil)
			if err != nil {
				log.Print("upgrade:", err)
				return
			}
			defer c.Close()
			for msg := range stream {
				if filter != "" && msg.Name != filter {
					continue
				}
				b, err := json.Marshal(msg)
				if err != nil {
					log.Println("encode:", err)
					break
				}
				if err = c.WriteMessage(websocket.TextMessage, b); err != nil {
					log.Println("write:", err)
					break
				}
			}
		})

		server := &http.Server{
			Addr:    ":0",
			Handler: mux,
		}
		ctx = oversight.WithContext(ctx, oversight.WithLogger(log.New(os.Stderr, "", log.LstdFlags)))
		oversight.Add(ctx, func(context.Context) error {
			if err := server.Serve(l); err != nil {
				log.Println("service discovery server failed:", err)
			}
			return err
		})
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()
	return nil
}