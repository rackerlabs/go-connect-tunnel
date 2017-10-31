/*
 *
 * Copyright 2017 Rackspace
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS-IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package tunnel_test

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/rackerlabs/go-connect-tunnel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestValidConnect(t *testing.T) {
	captureProxy := CaptureProxy{
		ResponseBody: "Expect this farend response\n",
	}

	testServerUrl, captures := captureProxy.Start()
	defer captureProxy.Stop(10 * time.Second)

	proxyUrl, err := url.Parse(testServerUrl)
	require.NoError(t, err)

	conn, err := tunnel.DialViaProxy(proxyUrl, "localhost:8080")
	require.NoError(t, err)
	assert.NotNil(t, conn)

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout")

	case c := <-captures:
		assert.Equal(t, "HTTP/1.1", c.Proto)
		assert.Equal(t, "CONNECT", c.Method)
		assert.Equal(t, "localhost:8080", c.Host)
	}

	fmt.Println("Reading tunnelled content")
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	require.NoError(t, err)
	assert.Equal(t, "Expect this farend response\n", line)

	conn.Write([]byte("Client sent content\n"))

	select {
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout")

	case c := <-captures:
		assert.Equal(t, "Client sent content\n", c.Body)
	}
}

type ExampleHandler struct{}

func (h *ExampleHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	log.Printf("REQUEST: method=%v, host=%v", req.Method, req.Host)
}

func ExampleDialViaProxy() {
	pretendProxy := httptest.NewServer(&ExampleHandler{})
	defer pretendProxy.Close()

	proxyUrl, err := url.Parse(pretendProxy.URL)
	if err != nil {
		log.Fatal(err)
	}

	// Dial a TCP connection to the host at "farend" on port 5000 via the HTTP proxy
	conn, err := tunnel.DialViaProxy(proxyUrl, "farend:5000")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	if conn != nil {
		fmt.Println("Connection ready to use")
	}
	// ...proceed with net.Conn operations
	// however, this particular conn won't actually work since the built-in HTTP server is not CONNECT aware

	// Output: Connection ready to use
}

type Capture struct {
	Body   string
	Proto  string
	Method string
	Host   string
}

type CaptureProxy struct {
	TestContext  *testing.T
	ResponseBody string
	captured     chan Capture
	httpServer   *httptest.Server
}

func (p *CaptureProxy) Start() (string, <-chan Capture) {
	p.httpServer = httptest.NewServer(p)

	p.captured = make(chan Capture, 1)

	return p.httpServer.URL, p.captured
}

func (p *CaptureProxy) Stop(timeout time.Duration) {
	p.httpServer.Close()
}

func (p *CaptureProxy) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	var bodyBuffer bytes.Buffer
	line, err := bodyBuffer.ReadString('\n')
	if err != nil && err != io.EOF {
		p.TestContext.Fatal("In ServeHTTP: ", err)
	}

	resp.WriteHeader(http.StatusOK)
	p.captured <- Capture{
		Body:   line,
		Proto:  req.Proto,
		Host:   req.Host,
		Method: req.Method,
	}

	conn, writer, err := resp.(http.Hijacker).Hijack()
	if err != nil {
		p.TestContext.Fatal("Hijacking: ", err)
	}
	writer.Write([]byte(p.ResponseBody))
	writer.Flush()

	reader := bufio.NewReader(conn)
	line, err = reader.ReadString('\n')
	if err != nil {
		p.TestContext.Fatal("Reading from hijacked: ", err)
	}
	p.captured <- Capture{
		Body: line,
	}
}
