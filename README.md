[![Go Documentation](https://godoc.org/github.com/rackerlabs/go-connect-tunnel?status.svg)](https://godoc.org/github.com/rackerlabs/go-connect-tunnel)
[![CircleCI](https://img.shields.io/circleci/project/github/rackerlabs/go-connect-tunnel.svg)](https://circleci.com/gh/rackerlabs/go-connect-tunnel)

## Example

```go
conn, err := tunnel.DialViaProxy(proxyUrl, "farend:5000")
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

if conn != nil {
    fmt.Println("Connection ready to use")
}
// ...proceed with net.Conn operations
```