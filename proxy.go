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

// Package tunnel provides a specialized version of net.Dial that can establish a tunnelled TCP connection via
// an HTTP proxy that supports the CONNECT method per RFC 2817.
package tunnel

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"github.com/pkg/errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	ProxyDeadlineDuration = 30 * time.Second
)

// DialViaProxy will establish a TCP connection tunnelled over an HTTP proxy located at the given url.
// The HTTP proxy must support unauthenticated CONNECT method tunnelling per RFC 2817.
// The farendAddress must be a host:port of the far-end TCP endpoint.
// The url must have a scheme of either http or https and explicitly specify the port.
func DialViaProxy(url *url.URL, farendAddress string) (net.Conn, error) {
	proxyAddress := net.JoinHostPort(url.Hostname(), url.Port())
        proxyAuth := ""
	if url.User != nil {
		proxyAuth = "Basic " + base64.StdEncoding.EncodeToString([]byte(url.User.String()))
	}

	switch url.Scheme {
	case "http":
		conn, err := net.Dial("tcp", proxyAddress)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to connect to proxy")
		}

		return establishProxyConnect(conn, farendAddress, proxyAuth)

	case "https":
		conn, err := tls.Dial("tcp", proxyAddress, nil)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to connect via TLS to proxy")
		}

		return establishProxyConnect(conn, farendAddress, proxyAuth)

	default:
		return nil, fmt.Errorf("URL scheme '%v' not supported for proxy", url.Scheme)
	}
}

func establishProxyConnect(conn net.Conn, farendAddress string, proxyAuth string) (net.Conn, error) {
	err := conn.SetDeadline(time.Now().Add(ProxyDeadlineDuration))
	if err != nil {
		return nil, err
	}
	req := ""
        if proxyAuth != "" {
	    req = fmt.Sprintf("CONNECT %s HTTP/1.1\nHost: %s\nProxy-Authorization: %s\n\n", farendAddress, farendAddress, proxyAuth)
        } else {
            req = fmt.Sprintf("CONNECT %s HTTP/1.1\nHost: %s\n\n", farendAddress, farendAddress)
        }
	_, err = conn.Write([]byte(req))
	if err != nil {
		return nil, err
	}

	// now the write side of the stream is positioned for tunneled writes

	readBuf := bufio.NewReader(conn)
	respLine, err := readBuf.ReadString('\n')
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read CONNECT response line")
	}

	// purposely using some very safe, but tedious means to inspect the response line

	if !strings.HasPrefix(respLine, "HTTP/1.1 ") {
		return nil, fmt.Errorf("unexpected CONNECT response line from proxy: %s", respLine)
	}

	pos := strings.IndexByte(respLine[9:], ' ')
	if pos == -1 {
		return nil, fmt.Errorf("invalid CONNECT response line from proxy: %s", respLine)
	}

	respCode, err := strconv.ParseInt(respLine[9:9+pos], 10, 16)
	if err != nil {
		return nil, fmt.Errorf("invalid CONNECT response code from proxy: %s", respLine)
	}

	if respCode != 200 {
		return nil, fmt.Errorf("expected 200 CONNECT response code but got %d", respCode)
	}

	var resetValue time.Time
	conn.SetDeadline(resetValue)

	return conn, nil
}
