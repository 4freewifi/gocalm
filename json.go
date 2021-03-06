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
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

// readJSON reads from http.Request, decode it as a JSON object into
// v, then return the read []byte and error if any.
func readJSON(v interface{}, r *http.Request) (b []byte, err error) {
	body := r.Body
	defer body.Close()
	b, err = ioutil.ReadAll(body)
	if err != nil {
		glog.Errorln(err)
		return
	}
	err = json.Unmarshal(b, v)
	if err != nil {
		glog.Warningln(err)
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
			glog.Warningf("Invalid Content-Type: %s\n", element)
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
