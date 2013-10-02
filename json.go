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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	ID = "id"
)

// paginateJSON expects `b' to be an JSON array, and return a slice
// that starts from the first item whose `id' is bigger then `last', and
// its length is limited to `limit'. For now, only id of type `number'
// and `string' are supported.
func paginateJSON(b []byte, last string, limit int) (
	p []byte, err error) {
	var v interface{}
	var i map[string]interface{}
	mismatch := errors.New(TypeMismatch)
	err = json.Unmarshal(b, &v)
	if err != nil {
		return
	}
	array, ok := v.([]interface{})
	if !ok {
		err = mismatch
		return
	}
	max := len(array)
	if max == 0 {
		return b, nil
	}
	i, ok = array[0].(map[string]interface{})
	var last_i64 int64
	var typ string
	// test type of "id"
	switch i[ID].(type) {
	case float64:
		if last == "" {
			last = "-1"
		}
		last_i64, err = strconv.ParseInt(last, 10, 64)
		if err != nil {
			return
		}
		typ = "float64"
	case string:
		typ = "string"
	default:
		err = errors.New("Unrecognized type: " + fmt.Sprint(i[ID]))
		return
	}
	var start int = 0
	for found := false; start < max; start++ {
		inconsistent := errors.New("Inconsistent type of id")
		i, ok = array[start].(map[string]interface{})
		if !ok {
			err = mismatch
			return
		}
		switch typ {
		case "string":
			id, ok := i["id"].(string)
			if !ok {
				err = inconsistent
				return
			}
			if id > last {
				found = true
			}
		case "float64":
			f, ok := i["id"].(float64)
			if !ok {
				err = inconsistent
				return
			}
			id := int64(f)
			if id > last_i64 {
				found = true
			}
		default:
			panic("Shouldn't be here")
		}
		if found {
			break
		}
	}
	if start == max {
		err = errors.New(NotFound)
		return
	}
	slice := make([]interface{}, 0, max)
	for c, i := 0, start; c < limit && i < max; {
		slice = append(slice, array[i])
		c++
		i++
	}
	p, err = json.Marshal(slice)
	if err != nil {
		return
	}
	return
}

// readJSON reads from http.Request, decode it as a JSON object into
// v, then return the read []byte and error if any.
func readJSON(v interface{}, r *http.Request) (b []byte, err error) {
	body := r.Body
	defer body.Close()
	b, err = ioutil.ReadAll(body)
	if err != nil {
		log.Println(err)
		return
	}
	err = json.Unmarshal(b, v)
	if err != nil {
		log.Println(err)
	}
	return
}

var mediaRange *regexp.Regexp
var lws *regexp.Regexp

// acceptJSON check the HTTP Accept header to see if application/json
// is accepted.
func acceptJSON(accept string) bool {
	accept = lws.ReplaceAllString(accept, "")
	elements := strings.Split(accept, ",")
	for _, element := range elements {
		match := mediaRange.FindStringSubmatch(element)
		if match == nil {
			log.Printf("Invalid Content-Type: %s\n", element)
			return false
		}
		atype := match[1]
		asubtype := match[2]
		if (atype == "*" || atype == "application") &&
			(asubtype == "*" || asubtype == "json") {
			return true
		}
	}
	return false
}

func init() {
	var err error

	// Accept         = "Accept" ":"
	//                  #( media-range [ accept-params ] )
	// media-range    = ( "*/*"
	//                  | ( type "/" "*" )
	//                  | ( type "/" subtype )
	//                  ) *( ";" parameter )
	// accept-params  = ";" "q" "=" qvalue *( accept-extension )
	// accept-extension = ";" token [ "=" ( token | quoted-string ) ]
	mediaRange, err = regexp.Compile(`([[:alnum:]\*]+)/([[:alnum:]\*]+).*`)
	if err != nil {
		panic(err)
	}

	// LWS            = [CRLF] 1*( SP | HT )
	lws, err = regexp.Compile(`[\r\n][ \t]+`)
	if err != nil {
		panic(err)
	}
}
