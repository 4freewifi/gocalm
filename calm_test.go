package gocalm

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/johncylee/goroute"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type IdValue struct {
	Id    string
	Value string
}

var dataStore map[string]string = map[string]string{
	"Peter": "Lemon",
	"Paul":  "Tree",
	"Mary":  "Very Pretty",
}

type Model struct {
}

func (m *Model) Get(id string) (v interface{}, err error) {
	log.Println("Get")
	for n, v := range dataStore {
		if n == id {
			return &IdValue{
				Id:    id,
				Value: v,
			}, nil
		}
	}
	return nil, NotFound
}

func (m *Model) GetAll() (v interface{}, err error) {
	log.Println("GetAll")
	r := make([]IdValue, len(dataStore))
	i := 0
	for id, v := range dataStore {
		r[i].Id = id
		r[i].Value = v
		i++
	}
	return r, nil
}

func (m *Model) Put(id string, v interface{}) (err error) {
	log.Println("Put")
	f, ok := v.(*IdValue)
	if !ok {
		return TypeMismatch
	}
	for n, _ := range dataStore {
		if n == id {
			dataStore[id] = f.Value
			return nil
		}
	}
	return NotFound
}

func (m *Model) PutAll(v interface{}) (err error) {
	log.Println("PutAll")
	a, ok := v.([]IdValue)
	if !ok {
		return TypeMismatch
	}
	dataStore = make(map[string]string)
	for _, f := range a {
		dataStore[f.Id] = f.Value
	}
	return nil
}

func (m *Model) Post(v interface{}) (err error) {
	log.Println("Post")
	f, ok := v.(*IdValue)
	if !ok {
		return TypeMismatch
	}
	if dataStore[f.Id] != "" {
		return errors.New("Already exists")
	}
	dataStore[f.Id] = f.Value
	return nil
}

func (m *Model) Delete(id string) (err error) {
	log.Println("Delete")
	for n, _ := range dataStore {
		if n == id {
			delete(dataStore, n)
			return nil
		}
	}
	return NotFound
}

func (m *Model) DeleteAll() (err error) {
	log.Println("DeleteAll")
	dataStore = map[string]string{}
	return nil
}

func Expect(t *testing.T, b io.ReadCloser, expect []byte) {
	defer b.Close()
	body, err := ioutil.ReadAll(b)
	if err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(body, expect) {
		t.Errorf("Expect: `%s', got: `%s'\n", expect, body)
		return
	}
	log.Printf("Got expected response: `%s'\n", expect)
}

func VerifyGet(t *testing.T, s *httptest.Server, id string) {
	resp, err := http.Get(s.URL + "/" + id)
	if err != nil {
		t.Error(err)
		return
	}
	v := dataStore[id]
	if v == "" {
		Expect(t, resp.Body, []byte("Not found\n"))
		return
	}
	j, _ := json.Marshal(IdValue{id, dataStore[id]})
	Expect(t, resp.Body, j)
}

func TestRestful(t *testing.T) {
	h := RESTHandler{
		Model:    &Model{},
		DataType: reflect.TypeOf(IdValue{}),
	}
	s := httptest.NewServer(goroute.Handle("/", `(?P<id>[[:alnum:]]*)`, &h))
	defer s.Close()
	// GET each
	for _, id := range []string{"Peter", "Paul", "Mary"} {
		VerifyGet(t, s, id)
	}
	// GET /
	resp, err := http.Get(s.URL)
	if err != nil {
		t.Error(err)
	} else {
		tmpIdValues := make([]IdValue, len(dataStore))
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		err = json.Unmarshal(body, &tmpIdValues)
		if err != nil {
			t.Fatal(err)
		}
		tmpDataStore := make(map[string]string)
		for _, v := range tmpIdValues {
			tmpDataStore[v.Id] = v.Value
		}
		if !reflect.DeepEqual(tmpDataStore, dataStore) {
			t.Error(err)
		}
		log.Println("All data retrieved correctly")
	}
	// PUT /Peter
	client := http.Client{}
	req, err := http.NewRequest(`PUT`, s.URL+"/Peter",
		strings.NewReader(`{"Value":"Orange!"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(`Content-Type`, `application/json`)
	resp, err = client.Do(req)
	if err != nil {
		t.Error(err)
	} else {
		Expect(t, resp.Body, []byte("OK"))
	}
	// GET /Peter to verify
	VerifyGet(t, s, "Peter")
	// POST
	j, _ := json.Marshal(IdValue{"JohnSmith", "Stranger"})
	req, err = http.NewRequest(`POST`, s.URL, bytes.NewReader(j))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(`Content-Type`, `application/json`)
	resp, err = client.Do(req)
	if err != nil {
		t.Error(err)
	} else {
		Expect(t, resp.Body, []byte("OK"))
	}
	// GET /JohnSmith to verify
	VerifyGet(t, s, "JohnSmith")
	// DELETE /Paul
	req, err = http.NewRequest(`DELETE`, s.URL+"/Paul", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Error(err)
	} else {
		Expect(t, resp.Body, []byte("OK"))
	}
	// GET /Paul to verify
	VerifyGet(t, s, "Paul")
	// DELETE /
	req, err = http.NewRequest(`DELETE`, s.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Error(err)
	} else {
		Expect(t, resp.Body, []byte("OK"))
	}
	// GET / to verify
	resp, err = http.Get(s.URL)
	if err != nil {
		t.Error(err)
	} else {
		Expect(t, resp.Body, []byte("[]"))
	}
}
