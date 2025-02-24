package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"time"
	"runtime"
	"os/exec"
)

type RequestData struct {
	URL string `json:"url"`
}

type ResponseData struct {
	DNSLookup       int64 `json:"dns_lookup_ms"`
	TCPConnect      int64 `json:"tcp_connect_ms"`
	TLSHandshake    int64 `json:"tls_handshake_ms"`
	TimeToFirstByte int64 `json:"time_to_first_byte_ms"`
	TotalTime       int64 `json:"total_time_ms"`
}

type Timing struct {
	DnsStart           int64
	DnsDone            int64
	ConnectStart       int64
	ConnectDone        int64
	TlsHandshakeStart  int64
	TlsHandshakeDone   int64
	FirstByteStart     int64
	FirstByteDone      int64
	TransferStart      int64
}

func flushDNS() error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ipconfig", "/flushdns")
	} else {
		cmd = exec.Command("systemd-resolve", "--flush-caches")
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to flush DNS: %v", err)
	}
	return nil
}

func traceURL(url string) (*ResponseData, error) {
	if err := flushDNS(); err != nil {
		return nil, fmt.Errorf("failed to flush DNS: %v", err)
	}

	timing := &Timing{}
	t0 := time.Now().UTC().UnixMilli()
	trace := &httptrace.ClientTrace{
		DNSStart:          func(_ httptrace.DNSStartInfo) { timing.DnsStart = time.Now().UTC().UnixMilli() },
		DNSDone:           func(_ httptrace.DNSDoneInfo) { timing.DnsDone = time.Now().UTC().UnixMilli() },
		ConnectStart:      func(_, _ string) { timing.ConnectStart = time.Now().UTC().UnixMilli() },
		ConnectDone:       func(_, _ string, _ error) { timing.ConnectDone = time.Now().UTC().UnixMilli() },
		TLSHandshakeStart: func() { timing.TlsHandshakeStart = time.Now().UTC().UnixMilli() },
		TLSHandshakeDone:  func(_ tls.ConnectionState, _ error) { timing.TlsHandshakeDone = time.Now().UTC().UnixMilli() },
		GotConn: func(_ httptrace.GotConnInfo) {
			timing.FirstByteStart = time.Now().UTC().UnixMilli()
		},
		GotFirstResponseByte: func() {
			timing.FirstByteDone = time.Now().UTC().UnixMilli()
			timing.TransferStart = time.Now().UTC().UnixMilli()
		},
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
	t1 := time.Now().UTC().UnixMilli()

	return &ResponseData{
		DNSLookup:       timing.DnsDone - timing.DnsStart,
		TCPConnect:      timing.ConnectDone - timing.ConnectStart,
		TLSHandshake:    timing.TlsHandshakeDone - timing.TlsHandshakeStart,
		TimeToFirstByte: timing.FirstByteDone - t0,
		TotalTime:       t1 - t0,
	}, nil
}


func Handler(w http.ResponseWriter, r *http.Request) {
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
