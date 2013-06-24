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

// gocalm is a RESTful service framework carefully designed to work
// with net/http and goroute but it is not tightly coupled to
// goroute. It is encouraged to store necessary data in self-defined
// context struct and keep the interface clean. Check the typical
// usage in calm_test.go .
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

const (
	// Default name of the key path parameter
	KEY string = "key"
)

var NotFound string = "Not found"
var TypeMismatch string = "Type mismatch"

type ModelInterface interface {
	// Get something that is suitable to json.Marshal. It does not
	// have to match RESTHandler.DataType.
	Get(key string) (v interface{}, err error)
	// GetAll returns something that is suitable to
	// json.Marshal. It does not have to match
	// RESTHandler.DataType. It can also be a channel and gocalm
	// will try to fetch object from it until it closes.
	GetAll() (v interface{}, err error)
	// Put `v' to replace object with key value `key'. The
	// original object must already exist, and `v' must be of type
	// RESTHandler.DataType.
	Put(key string, v interface{}) (err error)
	// PutAll replaces multiple objects.
	PutAll(v interface{}) (err error)
	// Post add object of type RESTHandler.DataType. It will
	// return the id of the newly added object.
	Post(v interface{}) (id string, err error)
	// Delete the object with key `key'.
	Delete(key string) (err error)
	// Delete every object.
	DeleteAll() (err error)
}

type ErrMsg struct {
	Message string `json:"message"`
}

// Sends http status code and message in json format
func sendJSONMsg(w http.ResponseWriter, status int, msg string) {
	j, err := json.Marshal(ErrMsg{msg})
	if err != nil {
		// that's enough reason to panic
		panic(err)
	}
	http.Error(w, string(j), status)
}

// sendNotFound sends 404
func sendNotFound(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s: %s: %s\n", r.Method, r.URL, NotFound)
	sendJSONMsg(w, http.StatusNotFound, NotFound)
}

// sendBadRequest sends 400 with given error message
func sendBadRequest(err interface{}, w http.ResponseWriter, r *http.Request) {
	msg := fmt.Sprint(err)
	log.Printf("%s: %s: %s\n", r.Method, r.URL, msg)
	sendJSONMsg(w, http.StatusBadRequest, msg)
}

// RESTHandler is http.Handler as well as goroute.Handler. It is
// designed to use with goroute but it is not tightly coupled.
type RESTHandler struct {
	// Name must be unique across all RESTHandlers
	Name string
	// Model is an interface to backend storage
	Model ModelInterface
	// reflect.TypeOf(<instance in model>)
	DataType reflect.Type
	// Used by memcached to determine expiration time in seconds
	Expiration int32
	// Keeps key-value pairs set by goroute
	PathParameters map[string]string
	sortedKeys     []string
}

// SetPathParameters is required by goroute, and gocalm.Key must exist
// inside path parameters for gocalm to work correctly.
func (h *RESTHandler) SetPathParameters(kvpairs map[string]string) {
	keys := make([]string, len(kvpairs))
	i := 0
	for k, _ := range kvpairs {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	h.sortedKeys = keys
	h.PathParameters = kvpairs
}

func (h *RESTHandler) getCacheKey(all bool) string {
	buf := bytes.NewBufferString(h.Name)
	for i := range h.sortedKeys {
		err := buf.WriteByte('_')
		if err != nil {
			panic(err)
		}
		if h.sortedKeys[i] == KEY && all {
			continue
		}
		_, err = buf.WriteString(h.PathParameters[h.sortedKeys[i]])
		if err != nil {
			panic(err)
		}
	}
	return buf.String()
}

// getJSON gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) getJSON(key string) ([]byte, error) {
	cacheKey := h.getCacheKey(false)
	item, err := MC.Get(cacheKey)
	if err == nil {
		log.Printf("memcache Get %s", cacheKey)
		return item.Value, nil
	}
	v, err := h.Model.Get(key)
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
	err = MC.Set(&memcache.Item{
		Key:        cacheKey,
		Value:      b,
		Expiration: h.Expiration,
	})
	if err == nil {
		log.Printf("memcache Set %s", cacheKey)
	} else {
		log.Printf("memcache Set %s: %s", cacheKey, err.Error())
	}
	return b, nil
}

// getAllJSON gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) getAllJSON() ([]byte, error) {
	cacheKey := h.getCacheKey(true)
	item, err := MC.Get(cacheKey)
	if err == nil {
		log.Printf("memcache Get %s", cacheKey)
		return item.Value, nil
	}
	v, err := h.Model.GetAll()
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
		// ignore error, if any.
		err = MC.Set(&memcache.Item{
			Key:        cacheKey,
			Value:      b,
			Expiration: h.Expiration,
		})
		if err == nil {
			log.Printf("memcache Set %s", cacheKey)
		} else {
			log.Printf("memcache Set %s: %s", cacheKey, err.Error())
		}
		return b, nil
	}
	c, ok := v.(chan interface{})
	if !ok {
		return nil, errors.New(
			"type must be chan interface{}")
	}
	buf := bytes.Buffer{}
	_, err = buf.Write([]byte{'['})
	if err != nil {
		return nil, err
	}
	i := 0
	for vv := range c {
		if err, ok := vv.(error); ok {
			return nil, err
		}
		if i != 0 {
			_, err = buf.Write([]byte{','})
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
	_, err = buf.Write([]byte{']'})
	if err != nil {
		return nil, err
	}
	b := buf.Bytes()
	err = MC.Set(&memcache.Item{
		Key:        cacheKey,
		Value:      b,
		Expiration: h.Expiration,
	})
	if err == nil {
		log.Printf("memcache Set %s", cacheKey)
	} else {
		log.Printf("memcache Set %s: %s", cacheKey, err.Error())
	}
	return b, nil
}

// deleteMCAll deletes the "list all" cache.
func (h *RESTHandler) deleteMCAll() {
	cacheKey := h.getCacheKey(true)
	err := MC.Delete(cacheKey)
	if err == nil {
		log.Printf("memcache Delete %s", cacheKey)
	}
}

// deleteMCKey deletes corresponding cache as well as the "list all"
// cache because it will certainly be outdated if an element changed.
func (h *RESTHandler) deleteMCKey() {
	cacheKey := h.getCacheKey(false)
	err := MC.Delete(cacheKey)
	if err == nil {
		log.Printf("memcache Delete %s", cacheKey)
	}
	h.deleteMCAll()
}

func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			msg := fmt.Sprint(err)
			log.Println(msg)
			sendJSONMsg(w, http.StatusInternalServerError, msg)
		}
	}()
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
		sendJSONMsg(w, http.StatusNotAcceptable,
			"Supported Content-Type: application/json")
		return
	}
	key := h.PathParameters[KEY]
	switch {
	case r.Method == "GET" && key != "":
		b, err := h.getJSON(key)
		if err != nil {
			panic(err)
		}
		if b == nil {
			sendNotFound(w, r)
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
	case r.Method == "GET":
		b, err := h.getAllJSON()
		if err != nil {
			panic(err)
		}
		if b == nil {
			sendNotFound(w, r)
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
	case r.Method == "PUT" && key != "":
		v := reflect.New(h.DataType).Interface()
		_, err := readJSON(v, r)
		if err != nil {
			sendBadRequest(err, w, r)
			return
		}
		err = h.Model.Put(key, v)
		if err != nil {
			sendBadRequest(err, w, r)
			return
		}
		h.deleteMCKey()
		sendJSONMsg(w, http.StatusOK, "Success")
	case r.Method == "PUT":
		// TODO: do not implement this until we have reflect.SliceOf
		sendBadRequest("Not implemented", w, r)
	case r.Method == "POST" && key == "":
		v := reflect.New(h.DataType).Interface()
		_, err := readJSON(v, r)
		if err != nil {
			sendBadRequest(err, w, r)
			return
		}
		id, err := h.Model.Post(v)
		if err != nil {
			sendBadRequest(err, w, r)
			return
		}
		h.deleteMCAll()
		fmt.Fprintf(w, `{"id": "%s"}`, id)
	case r.Method == "DELETE" && key != "":
		err := h.Model.Delete(key)
		if err != nil {
			sendNotFound(w, r)
			return
		}
		h.deleteMCKey()
		sendJSONMsg(w, http.StatusOK, "Success")
	case r.Method == "DELETE" && key == "":
		sendBadRequest("Not implemented", w, r)
	default:
		msg := fmt.Sprintf("Unsupported request method: %s", r.Method)
		sendBadRequest(msg, w, r)
		return
	}
	return
}
