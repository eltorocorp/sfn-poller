package util

import (
	"encoding/json"
	"log"
)

// MustMarshal attempts to marshal a struct to a json message.
// This function logs then panics if an error occurs while marshalling.
func MustMarshal(v interface{}) *string {
	b, err := json.Marshal(v)
	if err != nil {
		log.Println(err)
		panic(err)
	}
	s := string(b)
	return &s
}

// MustUnmarshal attempts to unmarshal json to a struct.
// This function logs then panics if an error occurs while unmarshalling.
func MustUnmarshal(buf *string, v interface{}) {
	err := json.Unmarshal([]byte(*buf), v)
	if err != nil {
		log.Println(err)
		panic(err)
	}
}
