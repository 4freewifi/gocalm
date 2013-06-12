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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bradfitz/gomemcache/memcache"
	"log"
	"net/http"
	"reflect"
)

var NotFound error = errors.New("Not found")
var TypeMismatch error = errors.New("Type mismatch")

type ModelInterface interface {
	Get(key string) (v interface{}, err error)
	GetAll() (v interface{}, err error)
	Put(key string, v interface{}) (err error)
	PutAll(v interface{}) (err error)
	Post(v interface{}) (err error)
	Delete(key string) (err error)
	DeleteAll() (err error)
}

// SendNotFound sends 404
func SendNotFound(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s: %s: %s\n", r.Method, r.URL, NotFound)
	http.Error(w, NotFound.Error(), http.StatusNotFound)
}

// SendBadRequest sends 400 with given error message
func SendBadRequest(
	err error, w http.ResponseWriter, r *http.Request) {
	log.Printf("%s: %s: %s\n", r.Method, r.URL, err)
	http.Error(w, err.Error(), http.StatusBadRequest)
}

type RESTHandler struct {
	// Name must be unique across all RESTHandlers
	Name string
	// Model is an interface to backend storage
	Model ModelInterface
	// reflect.TypeOf(<instance in model>)
	DataType reflect.Type
	// Used by memcached to determine expiration time in seconds
	Expiration     int32
	pathParameters map[string]string
}

// SetPathParameters is required by goroute
func (h *RESTHandler) SetPathParameters(nvpairs map[string]string) {
	h.pathParameters = nvpairs
}

// getJSON gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) getJSON(key string) ([]byte, error) {
	mckey := h.Name + "_" + key
	item, _ := MC.Get(mckey)
	if item != nil {
		log.Printf("memcache Get %s", mckey)
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
		Key:        mckey,
		Value:      b,
		Expiration: h.Expiration,
	})
	if err == nil {
		log.Printf("memcache Set %s", mckey)
	} else {
		log.Printf("memcache Set %s: %s", mckey, err.Error())
	}
	return b, nil
}

// getAllJSON gets value from memcache if it exists or gets it from Model
func (h *RESTHandler) getAllJSON() ([]byte, error) {
	mckey := h.Name + "__all"
	item, _ := MC.Get(mckey)
	if item != nil {
		log.Printf("memcache Get %s", mckey)
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
			Key:        mckey,
			Value:      b,
			Expiration: h.Expiration,
		})
		if err == nil {
			log.Printf("memcache Set %s", mckey)
		} else {
			log.Printf("memcache Set %s: %s", mckey, err.Error())
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
		Key:        mckey,
		Value:      b,
		Expiration: h.Expiration,
	})
	if err == nil {
		log.Printf("memcache Set %s", mckey)
	} else {
		log.Printf("memcache Set %s: %s", mckey, err.Error())
	}
	return b, nil
}
func (h *RESTHandler) deleteMCAll() {
	mckey := h.Name + "__all"
	err := MC.Delete(mckey)
	if err == nil {
		log.Printf("memcache Delete %s", mckey)
	} else {
		log.Printf("memcache Delete %s: %s", mckey, err.Error())
	}
}

func (h *RESTHandler) deleteMCKey(key string) {
	mckey := h.Name + "_" + key
	err := MC.Delete(mckey)
	if err == nil {
		log.Printf("memcache Delete %s", mckey)
	} else {
		log.Printf("memcache Delete %s: %s", mckey, err.Error())
	}
	h.deleteMCAll()
}

func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			log.Panicln(err)
			http.Error(w, fmt.Sprint(err),
				http.StatusInternalServerError)
		}
	}()
	accept_json := true
	accepts := r.Header["Accept"]
	if len(accepts) > 0 {
		accept_json = false
	}
	for _, accept := range accepts {
		if AcceptJSON(accept) {
			accept_json = true
			break
		}
	}
	if !accept_json {
		log.Printf("`%s' is not supported.\n", accepts)
		http.Error(w, "Supported Content-Type: application/json",
			http.StatusNotAcceptable)
		return
	}
	key := h.pathParameters["key"]
	switch {
	case r.Method == "GET" && key != "":
		b, err := h.getJSON(key)
		if err != nil {
			panic(err)
		}
		if b == nil {
			SendNotFound(w, r)
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
			SendNotFound(w, r)
		}
		_, err = w.Write(b)
		if err != nil {
			panic(err)
		}
	case r.Method == "PUT" && key != "":
		v := reflect.New(h.DataType).Interface()
		_, err := ReadJSON(v, r)
		if err != nil {
			SendBadRequest(err, w, r)
			return
		}
		err = h.Model.Put(key, v)
		if err != nil {
			SendBadRequest(err, w, r)
			return
		}
		h.deleteMCKey(key)
		fmt.Fprint(w, "OK")
	case r.Method == "PUT":
		// TODO: do not implement this until we have reflect.SliceOf
		panic(errors.New("Not implemented"))
	case r.Method == "POST" && key == "":
		v := reflect.New(h.DataType).Interface()
		_, err := ReadJSON(v, r)
		if err != nil {
			SendBadRequest(err, w, r)
			return
		}
		err = h.Model.Post(v)
		if err != nil {
			SendBadRequest(err, w, r)
			return
		}
		h.deleteMCAll()
		fmt.Fprint(w, "OK")
	case r.Method == "DELETE" && key != "":
		err := h.Model.Delete(key)
		if err != nil {
			SendNotFound(w, r)
			return
		}
		h.deleteMCKey(key)
		fmt.Fprint(w, "OK")
	case r.Method == "DELETE" && key == "":
		panic(errors.New("Not implemented"))
	default:
		err := fmt.Errorf("Unsupported request method: %s", r.Method)
		SendBadRequest(err, w, r)
		return
	}
	return
}
