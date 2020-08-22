package gemini_test

import (
	"bytes"
	"fmt"
	"io"

	"toast.cafe/x/gemini"
)

func ExampleResponse_streaming() {
	var con1 io.Reader // client connection to another server
	var con2 io.Writer // server connection to the requesting client

	var response gemini.Response
	response.FromReader(con1)

	con2.Write(response.Header())
	io.Copy(con2, &response)
}

func ExampleResponse_reading() {
	input := []byte("20 text/gemini\r\nmy body")
	reader := bytes.NewReader(input)

	var response gemini.Response
	response.FromReader(reader)
	body, _ := response.Body() // ignore error
	fmt.Printf("%d %s | %s", response.Status, response.Meta(), body)
	// Output: 20 text/gemini | my body
}

func ExampleResponse_writing() {
	response, _ := gemini.NewResponse(20, "my meta") // ignore error
	// you can also use gemini.StatusSuccess
	response.WriteString("my ")
	response.Write([]byte("body"))
	response.Flush()
	body, _ := response.Body() // ignore error
	fmt.Printf("%d %s | %s", response.Status, response.Meta(), body)
	// Output: 20 my meta | my body
}
