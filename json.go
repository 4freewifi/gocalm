// Copyright 2013 John Lee <john@0xlab.org>
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

package gocalm

import (
	"encoding/json"
	"log"
	"net/http"
)

func ReadJSON(v interface{}, r *http.Request) (err error) {
	body := r.Body
	defer body.Close()
	dec := json.NewDecoder(body)
	err = dec.Decode(v)
	if err != nil {
		log.Println(err)
	}
	return
}

func WriteJSON(v interface{}, w http.ResponseWriter) (err error) {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	_, err = w.Write(b)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	return
}
