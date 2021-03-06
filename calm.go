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

/*

gocalm is a RESTful service framework carefully designed to work with
net/http and goroute but it is not tightly coupled to goroute. It is
encouraged to store necessary data in self-defined context struct and
keep the interface clean. Check the typical usage in calm_test.go .

Introduce kvpairs

kvpairs is a map[string]string as an argument to communicate with
Model to specify the data to retrieve/modify. gocalm will also
automatically parse query values in URL to put into kvpairs. It will
overwrite existing values, so it's best not to use duplicated
parameter names.

*/
package gocalm

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/evanphx/json-patch"
	"github.com/golang/glog"
	"io/ioutil"
	"net/http"
	"reflect"
)

const (
	SUCCESS            = "Success"
	NOT_FOUND          = "Not Found"
	NOT_ALLOWED        = "Method Not Allowed"
	TYPE_MISMATCH      = "Type mismatch"
	MEMCACHE_KEY_MAX   = 250
	MEMCACHE_VALUE_MAX = 1000000
)

// error with http status code
type Error struct {
	StatusCode int    `json:"status"`
	Message    string `json:"message"`
}

func (t *Error) Error() string {
	var msg string
	if t.Message == "" {
		msg = http.StatusText(t.StatusCode)
	} else {
		msg = t.Message
	}
	return fmt.Sprintf("%d: %s", t.StatusCode, msg)
}

var ErrNotFound *Error = &Error{
	StatusCode: http.StatusNotFound,
	Message:    NOT_FOUND,
}

var ErrNotImplemented *Error = &Error{
	StatusCode: http.StatusMethodNotAllowed,
	Message:    NOT_ALLOWED,
}

var ErrTypeMismatch *Error = &Error{
	StatusCode: http.StatusBadRequest,
	Message:    TYPE_MISMATCH,
}

// ModelInterface feeds data to RESTHandler
type ModelInterface interface {

	// Get something that is suitable to json.Marshal. It does not
	// have to match RESTHandler.DataType.
	Get(kvpairs map[string]string) (v interface{}, err error)

	// GetAll returns something that is suitable to
	// json.Marshal. It does not have to match
	// RESTHandler.DataType. It can also be a channel and gocalm will
	// try to fetch object from it until it closes.
	GetAll(kvpairs map[string]string) (v interface{}, err error)

	// Put `v' to replace object specified by kvpairs. The
	// original object must already exist, and `v' must be a
	// pointer to an object of type RESTHandler.DataType.
	Put(kvpairs map[string]string, v interface{}) (err error)

	// PutAll replaces multiple objects.
	PutAll(kvpairs map[string]string, v interface{}) (err error)

	// Patch update the original object specified by kvpairs. Both
	// patched and original must be pointers to objects of type
	// RESTHandler.DataType.
	Patch(kvpairs map[string]string, original interface{},
		patched interface{}) (err error)

	// Post add object. v must be a pointer to an object of type
	// RESTHandler.DataType. It will return the id of the newly
	// added object.
	Post(kvpairs map[string]string, v interface{}) (id string, err error)

	// Delete the object specified by kvpairs.
	Delete(kvpairs map[string]string) (err error)

	// Delete every object.
	DeleteAll(kvpairs map[string]string) (err error)
}

// Msg is the standard format to return server message
type Msg struct {
	Message string `json:"message"`
}

// Sends http status code and message in json format
func sendJSONMsg(w http.ResponseWriter, r *http.Request, status int,
	msg string) {
	s := fmt.Sprintf("%s %s: %d %s", r.Method, r.URL, status, msg)
	switch {
	case status < 400:
		glog.Info(s)
	case status < 500:
		glog.Warning(s)
	default:
		glog.Error(s)
	}
	b, err := json.Marshal(Msg{msg})
	if err != nil {
		// that's enough reason to panic
		panic(err)
	}
	w.WriteHeader(status)
	w.Write(b)
}

// sendInternalError sends 500 with given error message
func sendInternalError(e error, w http.ResponseWriter, r *http.Request) {
	sendJSONMsg(w, r, http.StatusInternalServerError, e.Error())
}

// RESTHandler is http.Handler as well as goroute.Handler.
type RESTHandler struct {
	// Name must be unique across all RESTHandlers
	Name string
	// Model is an interface to backend storage
	Model ModelInterface
	// reflect.TypeOf(<instance in model>)
	DataType reflect.Type
	// Cache expiration time in seconds. 0 means no cache.
	Expiration int32
	// The name of the primary key in request path
	Key string
	// memcache client
	Cache *memcache.Client
}

func (h *RESTHandler) String() string {
	if h.DataType != nil {
		return fmt.Sprintf(
			"{Name: %s, Model: %s, DataType: %s}",
			h.Name,
			reflect.TypeOf(h.Model).String(),
			h.DataType.String(),
		)
	}
	return fmt.Sprintf(
		"{Name: %s, Model: %s, DataType: nil}",
		h.Name,
		reflect.TypeOf(h.Model).String(),
	)
}

func (h *RESTHandler) makeKey(r *http.Request) string {
	b := md5.Sum([]byte(r.URL.RequestURI()))
	return hex.EncodeToString(b[:])
}

func (h *RESTHandler) cacheGet(key string) []byte {
	item, err := h.Cache.Get(key)
	if err != nil {
		glog.V(1).Infof("memcache Get '%s' error: %v", key, err)
		return nil
	}
	glog.V(1).Infof("memcache Get '%s'", key)
	return item.Value
}

func (h *RESTHandler) cacheSet(key string, value []byte) {
	if len(value) > MEMCACHE_VALUE_MAX {
		glog.Warningf("Cannot cache, value too big: handler %s, key %s",
			h.String(), key)
		return
	}
	err := h.Cache.Set(&memcache.Item{
		Key:        key,
		Value:      value,
		Expiration: h.Expiration,
	})
	if err != nil {
		glog.V(1).Infof("memcache Set '%s' error: %v", key, err)
		return
	}
	glog.V(1).Infof("memcache Set '%s'", key)
	return
}

// cached gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) cached(key string, kvpairs map[string]string) (
	[]byte, error) {
	if h.Expiration != 0 {
		value := h.cacheGet(key)
		if value != nil {
			return value, nil
		}
	}
	v, err := h.Model.Get(kvpairs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrNotFound
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if h.Expiration == 0 {
		return b, nil
	}
	h.cacheSet(key, b)
	return b, nil
}

// getAllJSON gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) getAllJSON(key string, kvpairs map[string]string) (
	[]byte, error) {
	if h.Expiration != 0 {
		value := h.cacheGet(key)
		if value != nil {
			return value, nil
		}
	}
	v, err := h.Model.GetAll(kvpairs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrNotFound
	}
	// model may return a `chan interface{}' to send items one by
	// one, or return a slice with every item in it.
	if reflect.ValueOf(v).Kind() != reflect.Chan {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		if h.Expiration == 0 {
			return b, nil
		}
		h.cacheSet(key, b)
		return b, nil
	}
	c, ok := v.(chan interface{})
	if !ok {
		return nil, errors.New(
			"type must be chan interface{}")
	}
	// drain channel before return
	defer func() {
		for _ = range c {
		}
	}()
	buf := bytes.Buffer{}
	err = buf.WriteByte('[')
	if err != nil {
		return nil, err
	}
	i := 0
	for vv := range c {
		if err, ok := vv.(error); ok {
			return nil, err
		}
		if i != 0 {
			err = buf.WriteByte(',')
			if err != nil {
				return nil, err
			}
		}
		b, err := json.Marshal(vv)
		if err != nil {
			return nil, err
		}
		_, err = buf.Write(b)
		if err != nil {
			return nil, err
		}
		i++
	}
	err = buf.WriteByte(']')
	if err != nil {
		return nil, err
	}
	b := buf.Bytes()
	if h.Expiration == 0 {
		return b, nil
	}
	h.cacheSet(key, b)
	return b, nil
}

func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request,
	kvpairs map[string]string) {
	defer func() {
		err := recover()
		if err == nil {
			return
		}
		switch e := err.(type) {
		case *Error:
			sendJSONMsg(w, r, e.StatusCode, e.Message)
		case error:
			sendInternalError(e, w, r)
		default:
			sendInternalError(
				fmt.Errorf("Error: %v", err), w, r)
		}
	}()
	// set content type in response header
	header := w.Header()
	header.Set("Content-Type", "application/json; charset=utf-8")
	// check if request accept json
	accept_json := true
	accepts := r.Header["Accept"]
	if len(accepts) > 0 {
		accept_json = false
	}
	for _, accept := range accepts {
		if acceptJSON(accept) {
			accept_json = true
			break
		}
	}
	if !accept_json {
		glog.Warningf("`%s' is not supported.\n", accepts)
		sendJSONMsg(w, r, http.StatusNotAcceptable,
			"Supported Content-Type: application/json")
		return
	}
	// put the query values in URL into kvpairs
	values := r.URL.Query()
	for k, _ := range values {
		// only get the first value, overwrite existing key
		kvpairs[k] = values.Get(k)
	}
	key := kvpairs[h.Key]
	switch {
	case r.Method == http.MethodGet && key != "":
		cachekey := h.makeKey(r)
		b, err := h.cached(cachekey, kvpairs)
		if err != nil {
			panic(err)
		}
		if b == nil {
			panic(ErrNotFound)
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
	case r.Method == http.MethodGet:
		cachekey := h.makeKey(r)
		b, err := h.getAllJSON(cachekey, kvpairs)
		if err != nil {
			panic(err)
		}
		if b == nil {
			panic(ErrNotFound)
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
	case r.Method == http.MethodPut && key != "":
		v := reflect.New(h.DataType).Interface()
		_, err := readJSON(v, r)
		if err != nil {
			panic(err)
		}
		err = h.Model.Put(kvpairs, v)
		if err != nil {
			panic(err)
		}
		sendJSONMsg(w, r, http.StatusOK, SUCCESS)
	case r.Method == http.MethodPut:
		// TODO: do not implement this until we have reflect.SliceOf
		panic(ErrNotImplemented)
	case r.Method == http.MethodPatch && key != "":
		defer r.Body.Close()
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		patch, err := jsonpatch.DecodePatch(b)
		if err != nil {
			glog.Errorf("jsonpatch.DecodePatch: %v", err)
			panic(err)
		}
		original, err := h.Model.Get(kvpairs)
		if err != nil {
			glog.Errorf("h.Model.Get %v", err)
			panic(err)
		}
		if b, err = json.Marshal(original); err != nil {
			panic(err)
		}
		glog.V(1).Infof("original: %s", string(b))
		if b, err = patch.Apply(b); err != nil {
			panic(err)
		}
		glog.V(1).Infof("patched: %s", string(b))
		patched := reflect.New(h.DataType).Interface()
		if err = json.Unmarshal(b, patched); err != nil {
			panic(err)
		}
		if err = h.Model.Patch(kvpairs, original, patched); err != nil {
			panic(err)
		}
		sendJSONMsg(w, r, http.StatusOK, SUCCESS)
	case r.Method == http.MethodPost && key == "":
		v := reflect.New(h.DataType).Interface()
		_, err := readJSON(v, r)
		if err != nil {
			panic(err)
		}
		id, err := h.Model.Post(kvpairs, v)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(w, `{"id": "%s"}`, id)
	case r.Method == http.MethodDelete && key != "":
		err := h.Model.Delete(kvpairs)
		if err != nil {
			panic(err)
		}
		sendJSONMsg(w, r, http.StatusOK, SUCCESS)
	case r.Method == http.MethodDelete && key == "":
		panic(ErrNotImplemented)
	default:
		panic(ErrNotImplemented)
	}
	return
}
