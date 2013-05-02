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
