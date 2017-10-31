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