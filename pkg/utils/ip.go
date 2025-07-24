/*
 * Copyright (c) 2025, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package utils

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// GetIPAddress gets the IP address of the user with timeout and fallback services
func GetIPAddress() (string, error) {
	// List of IP lookup services to try in order of preference
	ipServices := []struct {
		url     string
		timeout time.Duration
	}{
		{"https://api.ipify.org?format=text", 5 * time.Second},
		{"https://ifconfig.me/ip", 5 * time.Second},
		{"https://icanhazip.com", 5 * time.Second},
		{"https://ident.me", 5 * time.Second},
	}

	// Create context with overall timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Try each service until one works
	for _, service := range ipServices {
		ip, err := getIPFromHTTPService(ctx, service.url, service.timeout)
		if err == nil && isValidPublicIP(ip) {
			return fmt.Sprintf("%s/32", ip), nil
		}
	}

	return "", fmt.Errorf("failed to get IP address from all services")
}

// getIPFromHTTPService attempts to get IP from a specific HTTP service
func getIPFromHTTPService(ctx context.Context, url string, timeout time.Duration) (string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request for %s: %v", url, err)
	}

	// Set user agent to avoid being blocked
	req.Header.Set("User-Agent", "Holodeck")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching IP from %s: %v", url, err)
	}
	defer resp.Body.Close() // nolint:errcheck, gosec, staticcheck

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from %s: %s", url, resp.Status)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading response from %s: %v", url, err)
	}

	// Clean the IP address (remove whitespace and newlines)
	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty response from %s", url)
	}

	return ip, nil
}

// isValidPublicIP validates that the IP is a valid public IPv4 address
func isValidPublicIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Must be IPv4
	if ip.To4() == nil {
		return false
	}

	// Must not be private, loopback, or link-local
	return !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}
