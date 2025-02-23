package main

import (
    "crypto/tls"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/http/httptrace"
    "time"
)

type RequestData struct {
    URL string `json:"url"`
}

type ResponseData struct {
    DNSLookup         int64 `json:"dns_lookup_ms"`
    TCPConnect        int64 `json:"tcp_connect_ms"`
    TLSHandshake      int64 `json:"tls_handshake_ms"`
    TimeToFirstByte   int64 `json:"time_to_first_byte_ms"`
    TotalTime         int64 `json:"total_time_ms"`
}

func traceURL(url string) (*ResponseData, error) {
    var dnsStart, dnsEnd, connectStart, connectEnd, tlsStart, tlsEnd, firstByte time.Time
    t0 := time.Now()
    trace := &httptrace.ClientTrace{
        DNSStart: func(httptrace.DNSStartInfo) { dnsStart = time.Now() },
        DNSDone: func(httptrace.DNSDoneInfo) { dnsEnd = time.Now() },
        ConnectStart: func(string, string) { connectStart = time.Now() },
        ConnectDone: func(string, string, error) { connectEnd = time.Now() },
        TLSHandshakeStart: func() { tlsStart = time.Now() },
        TLSHandshakeDone: func(tls.ConnectionState, error) { tlsEnd = time.Now() },
        GotFirstResponseByte: func() { firstByte = time.Now() },
    }

    req, err := http.NewRequest("GET", url, nil)
    if err != nil {
        return nil, err
    }
    req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    io.Copy(io.Discard, resp.Body)
    t1 := time.Now()

    return &ResponseData{
        DNSLookup:       dnsEnd.Sub(dnsStart).Milliseconds(),
        TCPConnect:      connectEnd.Sub(connectStart).Milliseconds(),
        TLSHandshake:    tlsEnd.Sub(tlsStart).Milliseconds(),
        TimeToFirstByte: firstByte.Sub(t0).Milliseconds(),
        TotalTime:       t1.Sub(t0).Milliseconds(),
    }, nil
}

func handler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
        return
    }

    var reqData RequestData
    if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    result, err := traceURL(reqData.URL)
    if err != nil {
        http.Error(w, fmt.Sprintf("Error tracing URL: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

func main() {
    http.HandleFunc("/trace", handler)
    http.ListenAndServe(":8080", nil)
}


