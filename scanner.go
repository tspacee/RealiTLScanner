package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

// ScanResult holds the result of a TLS scan for a single host.
type ScanResult struct {
	IP          string
	Port        string
	ServerName  string
	HasReality  bool
	CertSubject string
	Latency     time.Duration
	Error       error
}

// Scanner performs TLS/Reality detection scans.
type Scanner struct {
	Timeout    time.Duration
	Concurrent int
}

// NewScanner creates a Scanner with the given timeout and concurrency.
// Personal note: increased default-friendly timeout to 10s since many hosts
// I scan are geographically distant and 5s caused too many false negatives.
func NewScanner(timeout time.Duration, concurrent int) *Scanner {
	return &Scanner{
		Timeout:    timeout,
		Concurrent: concurrent,
	}
}

// Scan attempts a TLS handshake to the given address and returns a ScanResult.
func (s *Scanner) Scan(ip, port, serverName string) ScanResult {
	result := ScanResult{
		IP:   ip,
		Port: port,
	}

	addr := net.JoinHostPort(ip, port)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", addr, s.Timeout)
	if err != nil {
		result.Error = fmt.Errorf("tcp dial: %w", err)
		return result
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(s.Timeout))

	tlsCfg := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true, //nolint:gosec // intentional for scanning
		MinVersion:         tls.VersionTLS13,
	}

	tlsConn := tls.Client(conn, tlsCfg)
	if err := tlsConn.Handshake(); err != nil {
		// A failed TLS 1.3 handshake may still indicate a Reality endpoint.
		result.HasReality = isRealityFingerprint(err)
		result.Error = fmt.Errorf("tls handshake: %w", err)
		result.Latency = time.Since(start)
		return result
	}

	result.Latency = time.Since(start)
	state := tlsConn.ConnectionState()

	if len(state.PeerCertificates) > 0 {
		result.CertSubject = state.PeerCertificates[0].Subject.CommonName
	}

	result.ServerName = serverName
	result.HasReality = detectReality(state)
	return result
}

// ScanBatch scans a slice of targets concurrently and returns results.
func (s *Scanner) ScanBatch(targets []Target) []ScanResult {
	sem := make(chan struct{}, s.Concurrent)
	resultCh := make(chan ScanResult, len(targets))

	for _, t := range targets {
		sem <- struct{}{}
		go func(t Target) {
			defer func() { <-sem }()
			resultCh <- s.Scan(t.IP, t.Port, t.ServerName)
		}(t)
	}

	// Drain semaphore to wait for all goroutines.
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}
	close(resultCh)

	results := make([]ScanResult, 0, len(targets))
	for r := range resultCh {
		results = append(results, r)
	}
	return results
}

// Target represents a host/port/SNI combination to scan.
type Target struct {
	IP         string
	Port       string
	ServerName string
}

// detectReality checks TLS state heuristics that suggest a REALITY endpoint.
// Note: self-signed certs on TLS 1.3 are the main signal here; not foolproof.
func detectReality(state tls.ConnectionState) bool {
	// REALITY endpoints typically present TLS 1.3 with no valid CA chain.
	if state.Version != tls.VersionTLS13 {
		return false
	}
	if len(state.VerifiedChains) == 0 {
		return true
	}
	return false
}
