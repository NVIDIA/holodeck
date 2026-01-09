/*
 * Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
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
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IP Utilities", func() {

	Describe("isValidPublicIP", func() {
		DescribeTable("validating IP addresses",
			func(ip string, expected bool) {
				result := isValidPublicIP(ip)
				Expect(result).To(Equal(expected))
			},
			Entry("valid public IPv4", "203.0.113.1", true),
			Entry("valid public IPv4 (8.8.8.8)", "8.8.8.8", true),
			Entry("valid public IPv4 (1.1.1.1)", "1.1.1.1", true),
			Entry("private 10.x.x.x", "10.0.0.1", false),
			Entry("private 10.255.x.x", "10.255.255.255", false),
			Entry("private 172.16.x.x", "172.16.0.1", false),
			Entry("private 172.31.x.x", "172.31.255.255", false),
			Entry("private 192.168.x.x", "192.168.1.1", false),
			Entry("private 192.168.0.x", "192.168.0.1", false),
			Entry("localhost 127.0.0.1", "127.0.0.1", false),
			Entry("localhost 127.0.0.2", "127.0.0.2", false),
			Entry("link-local 169.254.x.x", "169.254.1.1", false),
			Entry("empty string", "", false),
			Entry("invalid format", "not-an-ip", false),
			Entry("invalid numbers", "256.256.256.256", false),
			Entry("IPv6 loopback", "::1", false),
			Entry("IPv6 address", "2001:db8::1", false),
			Entry("partial IP", "192.168", false),
			Entry("IP with port", "8.8.8.8:53", false),
		)
	})

	Describe("getIPFromHTTPService", func() {
		var (
			server *httptest.Server
			ctx    context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		Context("when HTTP service returns valid IP", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("203.0.113.1")) //nolint:errcheck // test
					}))
			})

			It("should return the IP address", func() {
				ip, err := getIPFromHTTPService(ctx, server.URL, 5*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(ip).To(Equal("203.0.113.1"))
			})
		})

		Context("when HTTP service returns IP with whitespace", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("  203.0.113.1\n")) //nolint:errcheck // test
					}))
			})

			It("should trim whitespace and return the IP", func() {
				ip, err := getIPFromHTTPService(ctx, server.URL, 5*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(ip).To(Equal("203.0.113.1"))
			})
		})

		Context("when HTTP service returns non-200 status", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
					}))
			})

			It("should return an error", func() {
				_, err := getIPFromHTTPService(ctx, server.URL, 5*time.Second)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unexpected status"))
			})
		})

		Context("when HTTP service returns empty response", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						// Empty body
					}))
			})

			It("should return an error", func() {
				_, err := getIPFromHTTPService(ctx, server.URL, 5*time.Second)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("empty response"))
			})
		})

		Context("when HTTP service returns whitespace-only response", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("   \n  ")) //nolint:errcheck // test
					}))
			})

			It("should return an error", func() {
				_, err := getIPFromHTTPService(ctx, server.URL, 5*time.Second)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("empty response"))
			})
		})

		Context("when context is cancelled", func() {
			It("should return an error", func() {
				cancelledCtx, cancel := context.WithCancel(ctx)
				cancel()

				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						time.Sleep(100 * time.Millisecond)
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("203.0.113.1")) //nolint:errcheck // test
					}))

				_, err := getIPFromHTTPService(cancelledCtx, server.URL, 5*time.Second)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when URL is invalid", func() {
			It("should return an error", func() {
				_, err := getIPFromHTTPService(ctx, "://invalid-url", 5*time.Second)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error"))
			})
		})

		Context("when server is unreachable", func() {
			It("should return an error", func() {
				// Use a port that's definitely not listening
				_, err := getIPFromHTTPService(ctx,
					"http://127.0.0.1:59999/unreachable", 1*time.Second)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when User-Agent header is set", func() {
			var receivedUserAgent string

			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						receivedUserAgent = r.Header.Get("User-Agent")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("203.0.113.1")) //nolint:errcheck // test
					}))
			})

			It("should send Holodeck as User-Agent", func() {
				_, err := getIPFromHTTPService(ctx, server.URL, 5*time.Second)
				Expect(err).NotTo(HaveOccurred())
				Expect(receivedUserAgent).To(Equal("Holodeck"))
			})
		})
	})
})
