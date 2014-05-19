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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"log"
	"net/http"
	"reflect"
	"sort"
)

var Success string = "Success"
var TypeMismatch string = "Type mismatch"
var NotFound string = "Not found"
var ErrNotFound error = errors.New(NotFound)
var NotImplemented string = "Not implemented"
var ErrNotImplemented error = errors.New(NotImplemented)

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
	// original object must already exist, and `v' must be of type
	// RESTHandler.DataType.
	Put(kvpairs map[string]string, v interface{}) (err error)

	// PutAll replaces multiple objects.
	PutAll(kvpairs map[string]string, v interface{}) (err error)

	// Patch update object specified by kvpairs. The original
	// object must already exist, and `v' must be of type
	// RESTHandler.DataType, while the type of m should be
	// map[string]interface{} as returned by unmarshalling into an
	// interface{}.
	Patch(kvpairs map[string]string, v interface{},
		m map[string]interface{}) (err error)

	// Post add object of type RESTHandler.DataType. It will
	// return the id of the newly added object.
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
	log.Printf("%s %s: %d %s\n", r.Method, r.URL, status, msg)
	b, err := json.Marshal(Msg{msg})
	if err != nil {
		// that's enough reason to panic
		panic(err)
	}
	w.WriteHeader(status)
	w.Write(b)
}

// sendNotFound sends 404
func sendNotFound(w http.ResponseWriter, r *http.Request) {
	sendJSONMsg(w, r, http.StatusNotFound, NotFound)
}

// sendBadRequest sends 400 with given error message
func sendBadRequest(err interface{}, w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprint(err)
	sendJSONMsg(w, r, http.StatusBadRequest, msg)
}

// sendInternalError sends 500 with given error message
func sendInternalError(err interface{}, w http.ResponseWriter,
	r *http.Request) {
	msg := fmt.Sprint(err)
	sendJSONMsg(w, r, http.StatusInternalServerError, msg)
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

func (h *RESTHandler) getCacheKey(keys []string, kvpairs map[string]string,
) string {
	buf := bytes.NewBufferString(h.Name)
	for i := range keys {
		err := buf.WriteByte('_')
		if err != nil {
			panic(err)
		}
		_, err = buf.WriteString(kvpairs[keys[i]])
		if err != nil {
			panic(err)
		}
	}
	return buf.String()
}

// getJSON gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) getJSON(keys []string, kvpairs map[string]string) (
	[]byte, error) {
	var cacheKey string
	if h.Expiration != 0 {
		cacheKey = h.getCacheKey(keys, kvpairs)
		item, err := h.Cache.Get(cacheKey)
		if err == nil {
			log.Printf("memcache Get `%s'", cacheKey)
			return item.Value, nil
		}
	}
	v, err := h.Model.Get(kvpairs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil // found nothing. not an error.
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if h.Expiration == 0 {
		return b, nil
	}
	err = h.Cache.Set(&memcache.Item{
		Key:        cacheKey,
		Value:      b,
		Expiration: h.Expiration,
	})
	if err == nil {
		log.Printf("memcache Set `%s'", cacheKey)
	} else {
		log.Printf("memcache Set `%s': %s", cacheKey, err.Error())
	}
	return b, nil
}

// getAllJSON gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) getAllJSON(keys []string, kvpairs map[string]string) (
	[]byte, error) {
	var cacheKey string
	if h.Expiration != 0 {
		cacheKey = h.getCacheKey(keys, kvpairs)
		item, err := h.Cache.Get(cacheKey)
		if err == nil {
			log.Printf("memcache Get `%s'", cacheKey)
			return item.Value, nil
		}
	}
	v, err := h.Model.GetAll(kvpairs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil // found nothing. not an error.
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
		// ignore error, if any.
		err = h.Cache.Set(&memcache.Item{
			Key:        cacheKey,
			Value:      b,
			Expiration: h.Expiration,
		})
		if err == nil {
			log.Printf("memcache Set `%s'", cacheKey)
		} else {
			log.Printf("memcache Set `%s': %s", cacheKey,
				err.Error())
		}
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
	err = h.Cache.Set(&memcache.Item{
		Key:        cacheKey,
		Value:      b,
		Expiration: h.Expiration,
	})
	if err == nil {
		log.Printf("memcache Set `%s'", cacheKey)
	} else {
		log.Printf("memcache Set `%s': %s", cacheKey, err.Error())
	}
	return b, nil
}

// deleteCache deletes corresponding cache values.
func (h *RESTHandler) deleteCache(keys []string, kvpairs map[string]string) {
	cacheKey := h.getCacheKey(keys, kvpairs)
	err := h.Cache.Delete(cacheKey)
	if err == nil {
		log.Printf("memcache Delete `%s'", cacheKey)
	}
	if kvpairs[h.Key] == "" {
		return
	}
	// this means the `list all' cache has to be deleted as well.
	m := make(map[string]string)
	for k, v := range kvpairs {
		m[k] = v
	}
	m[h.Key] = ""
	cacheKey = h.getCacheKey(keys, m)
	err = h.Cache.Delete(cacheKey)
	if err == nil {
		log.Printf("memcache Delete `%s'", cacheKey)
	}
}

func errorHandler(err error, w http.ResponseWriter, r *http.Request) {
	switch err {
	case ErrNotFound:
		sendNotFound(w, r)
	case ErrNotImplemented:
		sendJSONMsg(w, r, http.StatusNotImplemented, NotImplemented)
	default:
		sendBadRequest(err, w, r)
	}
}

func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request,
	kvpairs map[string]string) {
	defer func() {
		if err := recover(); err != nil {
			sendInternalError(err, w, r)
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
		log.Printf("`%s' is not supported.\n", accepts)
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
	// make a sorted index of key names
	keys := make([]string, len(kvpairs))
	i := 0
	for k, _ := range kvpairs {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	key := kvpairs[h.Key]
	switch {
	case r.Method == "GET" && key != "":
		b, err := h.getJSON(keys, kvpairs)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		if b == nil {
			sendNotFound(w, r)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
	case r.Method == "GET":
		b, err := h.getAllJSON(keys, kvpairs)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		if b == nil {
			sendNotFound(w, r)
			return
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
	case r.Method == "PUT" && key != "":
		v := reflect.New(h.DataType).Interface()
		_, err := readJSON(v, r)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		err = h.Model.Put(kvpairs, v)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		if h.Expiration != 0 {
			h.deleteCache(keys, kvpairs)
		}
		sendJSONMsg(w, r, http.StatusOK, Success)
	case r.Method == "PUT":
		// TODO: do not implement this until we have reflect.SliceOf
		errorHandler(ErrNotImplemented, w, r)
	case r.Method == "PATCH" && key != "":
		v := reflect.New(h.DataType).Interface()
		b, err := readJSON(v, r)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		var i interface{}
		err = json.Unmarshal(b, &i)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		m, ok := i.(map[string]interface{})
		if !ok {
			sendBadRequest(TypeMismatch, w, r)
			return
		}
		err = h.Model.Patch(kvpairs, v, m)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		if h.Expiration != 0 {
			h.deleteCache(keys, kvpairs)
		}
		sendJSONMsg(w, r, http.StatusOK, Success)
	case r.Method == "POST" && key == "":
		v := reflect.New(h.DataType).Interface()
		_, err := readJSON(v, r)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		id, err := h.Model.Post(kvpairs, v)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		if h.Expiration != 0 {
			h.deleteCache(keys, kvpairs)
		}
		fmt.Fprintf(w, `{"id": "%s"}`, id)
	case r.Method == "DELETE" && key != "":
		err := h.Model.Delete(kvpairs)
		if err != nil {
			errorHandler(err, w, r)
			return
		}
		if h.Expiration != 0 {
			h.deleteCache(keys, kvpairs)
		}
		sendJSONMsg(w, r, http.StatusOK, Success)
	case r.Method == "DELETE" && key == "":
		errorHandler(ErrNotImplemented, w, r)
	default:
		msg := fmt.Sprintf("Unsupported request method: %s", r.Method)
		sendBadRequest(msg, w, r)
		return
	}
	return
}
