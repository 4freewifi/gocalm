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
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/johncylee/goroute"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const (
	KEY = "key"
)

type KeyValue struct {
	Key   int64  `json:"id"`
	Value string `json:"value"`
}

var dataStore map[int64]string = map[int64]string{
	0: "Peter",
	1: "Paul",
	2: "Mary",
}

type Model struct {
}

func (m *Model) Get(kvpairs map[string]string) (interface{}, error) {
	key, err := strconv.ParseInt(kvpairs[KEY], 10, 64)
	if err != nil {
		return nil, err
	}
	s := dataStore[key]
	if s == "" {
		return nil, nil // Not found
	}
	return &KeyValue{
		Key:   key,
		Value: s,
	}, nil
}

func (m *Model) GetAll(kvpairs map[string]string) (interface{}, error) {
	c := make(chan interface{})
	go func() {
		for n, v := range dataStore {
			c <- &KeyValue{Key: n, Value: v}
		}
		close(c)
	}()
	return c, nil
}

func (m *Model) Put(kvpairs map[string]string, v interface{}) (err error) {
	f, ok := v.(*KeyValue)
	if !ok {
		return errors.New(TypeMismatch)
	}
	f.Key, err = strconv.ParseInt(kvpairs[KEY], 10, 64)
	if err != nil {
		return
	}
	if _, ok := dataStore[f.Key]; !ok {
		return errors.New(NotFound)
	}
	dataStore[f.Key] = f.Value
	return nil
}

func (m *Model) PutAll(kvpairs map[string]string, v interface{}) (err error) {
	a, ok := v.([]KeyValue)
	if !ok {
		return errors.New(TypeMismatch)
	}
	dataStore = make(map[int64]string)
	for _, f := range a {
		dataStore[f.Key] = f.Value
	}
	return nil
}

func (m *Model) Patch(kvpairs map[string]string, v map[string]interface{}) (
	err error) {
	key, err := strconv.ParseInt(kvpairs[KEY], 10, 64)
	value := v[`value`].(string)
	dataStore[key] = value
	return nil
}

func (m *Model) Post(kvpairs map[string]string, v interface{}) (string, error) {
	f, ok := v.(*KeyValue)
	if !ok {
		return "", errors.New(TypeMismatch)
	}
	if _, ok := dataStore[f.Key]; ok {
		return "", errors.New("Already exists")
	}
	dataStore[f.Key] = f.Value
	return strconv.FormatInt(f.Key, 10), nil
}

func (m *Model) Delete(kvpairs map[string]string) (err error) {
	key, err := strconv.ParseInt(kvpairs[KEY], 10, 64)
	if err != nil {
		return
	}
	if dataStore[key] == "" {
		return errors.New(NotFound)
	}
	delete(dataStore, key)
	return nil
}

func (m *Model) DeleteAll(kvpairs map[string]string) (err error) {
	dataStore = map[int64]string{}
	return nil
}

func Expect(t *testing.T, r *http.Response, v interface{}) {
	switch expect := v.(type) {
	case []byte:
		b := r.Body
		defer b.Close()
		body, err := ioutil.ReadAll(b)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(body, expect) {
			t.Fatalf("Expect: `%s', got: `%s'\n", expect, body)
		}
		log.Printf("Got expected response: `%s'\n", expect)
		return
	case int:
		if r.StatusCode != expect {
			t.Fatalf("Expect status %d, got %d\n",
				expect, r.StatusCode)
		}
		log.Printf("Got expected status: %d\n", expect)
		return
	}
	t.Fatal("Unexpected type")
}

func VerifyGet(t *testing.T, s *httptest.Server, key string) {
	id, err := strconv.ParseInt(key, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	client := http.Client{}
	req, err := http.NewRequest(`GET`, s.URL+`/`+key, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(`Accept`, `application/json`)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	v := dataStore[id]
	if v == "" {
		Expect(t, res, http.StatusNotFound)
		return
	}
	j, _ := json.Marshal(KeyValue{id, dataStore[id]})
	Expect(t, res, j)
	req.Header.Set(`Accept`, `text/html`)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	Expect(t, res, http.StatusNotAcceptable)
}

func TestRestful(t *testing.T) {
	h := RESTHandler{
		Name:       "test",
		Model:      &Model{},
		DataType:   reflect.TypeOf(KeyValue{}),
		Expiration: 5, // expires in 5 seconds
		Key:        KEY,
		Cache:      memcache.New("127.0.0.1:11211"),
	}
	s := httptest.NewServer(goroute.Handle(
		"/", `(?P<key>[[:alnum:]]*)`, &h))
	defer s.Close()
	// GET each
	for _, id := range []string{"0", "1", "2"} {
		VerifyGet(t, s, id)
	}
	// GET /
	res, err := http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	tmpKeyValues := make([]KeyValue, len(dataStore))
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	err = json.Unmarshal(body, &tmpKeyValues)
	if err != nil {
		t.Fatal(err)
	}
	tmpDataStore := make(map[int64]string)
	for _, v := range tmpKeyValues {
		tmpDataStore[v.Key] = v.Value
	}
	if reflect.DeepEqual(tmpDataStore, dataStore) {
		log.Println("All data retrieved correctly")
	} else {
		t.Fatalf("%s != %s", tmpDataStore, dataStore)
	}
	// PUT /0
	client := http.Client{}
	req, err := http.NewRequest(`PUT`, s.URL+"/0",
		strings.NewReader(`{"value":"John"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(`Content-Type`, `application/json`)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	} else {
		Expect(t, res, 200)
	}
	// GET /0 to verify
	VerifyGet(t, s, "0")
	// POST
	j, _ := json.Marshal(KeyValue{3, "unknown"})
	req, err = http.NewRequest(`POST`, s.URL, bytes.NewReader(j))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(`Content-Type`, `application/json`)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	} else {
		Expect(t, res, 200)
	}
	// GET /3 to verify
	VerifyGet(t, s, "3")
	// PATCH
	req, err = http.NewRequest(`PATCH`, s.URL+`/3`, strings.NewReader(
		`{"value":"Mysterious Stranger"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(`Content-Type`, `application/json`)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	} else {
		Expect(t, res, 200)
	}
	// GET /3 to verify
	VerifyGet(t, s, "3")
	// DELETE /1
	req, err = http.NewRequest(`DELETE`, s.URL+"/1", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	} else {
		Expect(t, res, 200)
	}
	// GET /1 to verify
	VerifyGet(t, s, "1")
	// GET /
	res, err = http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, err = ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var v interface{}
	err = json.Unmarshal(body, &v)
	if err != nil {
		t.Fatal(err)
	}
	array, ok := v.([]interface{})
	if !ok {
		t.Fatal("type assertion failed: " + string(body))
	}
	if len(array) != 3 {
		t.Fatalf("expect 3 but items count is %d", len(array))
	}
	// DELETE /{0, 2, 3}
	for _, id := range []string{"0", "2", "3"} {
		req, err = http.NewRequest(`DELETE`, s.URL+"/"+id, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		} else {
			Expect(t, res, 200)
		}
	}
	// GET / to verify
	res, err = http.Get(s.URL)
	if err != nil {
		t.Fatal(err)
	} else {
		Expect(t, res, 200)
	}
}
