package utils

import (
	jsoniter "github.com/json-iterator/go"
)

type byteArray []byte

func (t byteArray) String() string {
	return string(t)
}

func (t byteArray) Bytes() []byte {
	return t
}

func ToJSON(v any) byteArray {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, _ := json.Marshal(v)

	return data
}

func ToIndentJSON(v any) byteArray {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	data, _ := json.MarshalIndent(v, "", "    ")

	return data
}

func FromJSON(data []byte, v any) (err error) {
	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	err = json.Unmarshal(data, v)
	return
}
