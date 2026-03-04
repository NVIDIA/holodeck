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

package gitref_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/NVIDIA/holodeck/pkg/gitref"
)

var _ = Describe("GitHubResolver", func() {
	var (
		resolver *gitref.GitHubResolver
		server   *httptest.Server
		ctx      context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
	})

	Describe("NewGitHubResolverWithClient", func() {
		It("should accept a custom HTTP client", func() {
			client := &http.Client{Timeout: 5 * time.Second}
			resolver = gitref.NewGitHubResolverWithClient(client)
			Expect(resolver).NotTo(BeNil())
		})
	})

	Describe("Resolve", func() {
		Context("with successful resolution", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{
							"sha": "abc123def456789012345678901234567890abcd"
						}`))
					}))
				resolver = gitref.NewGitHubResolverWithClient(server.Client())
			})

			It("should return full and short SHA for valid ref", func() {
				// Override the resolver to use our test server
				// We need to intercept the HTTP call
				server.Close()
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						Expect(r.Header.Get("Accept")).To(
							Equal("application/vnd.github.v3+json"))
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{
							"sha": "abc123def456789012345678901234567890abcd"
						}`))
					}))

				// Create a custom client that redirects to our server
				client := &http.Client{
					Transport: &rewriteTransport{
						server: server,
					},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				fullSHA, shortSHA, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/nvidia-container-toolkit.git",
					"v1.17.3",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).To(Equal("abc123def456789012345678901234567890abcd"))
				Expect(shortSHA).To(Equal("abc123de"))
			})

			It("should truncate short SHA to 8 characters", func() {
				server.Close()
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{
							"sha": "1234567890abcdef1234567890abcdef12345678"
						}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				fullSHA, shortSHA, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).To(HaveLen(40))
				Expect(shortSHA).To(HaveLen(8))
				Expect(shortSHA).To(Equal("12345678"))
			})

			It("should handle short SHA in response (< 8 chars)", func() {
				server.Close()
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						// Edge case: very short SHA (shouldn't happen but test it)
						_, _ = w.Write([]byte(`{"sha": "abc123"}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				fullSHA, shortSHA, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"abc123",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).To(Equal("abc123"))
				// Short SHA should be same as full when < 8 chars
				Expect(shortSHA).To(Equal("abc123"))
			})
		})

		Context("with invalid repository URL", func() {
			BeforeEach(func() {
				resolver = gitref.NewGitHubResolver()
			})

			It("should return error for GitLab URL", func() {
				_, _, err := resolver.Resolve(
					ctx,
					"https://gitlab.com/NVIDIA/toolkit.git",
					"main",
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GitHub repo URL"))
			})

			It("should return error for empty URL", func() {
				_, _, err := resolver.Resolve(ctx, "", "main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GitHub repo URL"))
			})

			It("should return error for malformed URL", func() {
				_, _, err := resolver.Resolve(ctx, "not-a-url", "main")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid GitHub repo URL"))
			})
		})

		Context("with HTTP errors", func() {
			It("should return error on HTTP 404", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"nonexistent-ref",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ref not found"))
				Expect(err.Error()).To(ContainSubstring("404"))
			})

			It("should return error on HTTP 429 rate limit", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusTooManyRequests)
						_, _ = w.Write([]byte(
							`{"message": "API rate limit exceeded"}`,
						))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("429"))
				Expect(err.Error()).To(ContainSubstring("rate limit"))
			})

			It("should return error on HTTP 500", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusInternalServerError)
						_, _ = w.Write([]byte(`{"message": "Internal error"}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("500"))
			})

			It("should handle empty error body", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusForbidden)
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("403"))
				Expect(err.Error()).To(ContainSubstring("no additional details"))
			})
		})

		Context("with malformed responses", func() {
			It("should return error on invalid JSON", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`not valid json`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to decode"))
			})

			It("should return error on empty SHA in response", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"sha": ""}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("empty SHA"))
			})

			It("should return error on missing SHA field", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"other": "field"}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("empty SHA"))
			})
		})

		Context("with network failures", func() {
			It("should return error when server is unreachable", func() {
				// Create a client that will fail to connect
				client := &http.Client{
					Transport: &failingTransport{},
					Timeout:   100 * time.Millisecond,
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to resolve ref"))
			})

			It("should respect context cancellation", func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						// Simulate slow response
						time.Sleep(500 * time.Millisecond)
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"sha": "abc123"}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)

				cancelCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()

				_, _, err := resolver.Resolve(
					cancelCtx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("with various ref formats", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{
							"sha": "fedcba9876543210fedcba9876543210fedcba98"
						}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)
			})

			It("should resolve tag reference", func() {
				fullSHA, shortSHA, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"v1.0.0",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).NotTo(BeEmpty())
				Expect(shortSHA).To(HaveLen(8))
			})

			It("should resolve branch reference", func() {
				fullSHA, shortSHA, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).NotTo(BeEmpty())
				Expect(shortSHA).To(HaveLen(8))
			})

			It("should resolve refs/tags/ format", func() {
				fullSHA, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"refs/tags/v1.0.0",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).NotTo(BeEmpty())
			})

			It("should resolve refs/heads/ format", func() {
				fullSHA, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"refs/heads/main",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).NotTo(BeEmpty())
			})

			It("should resolve PR ref format", func() {
				fullSHA, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"refs/pull/123/head",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).NotTo(BeEmpty())
			})

			It("should resolve commit SHA", func() {
				fullSHA, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"abc123def456",
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(fullSHA).NotTo(BeEmpty())
			})
		})

		Context("with various repo URL formats", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{
							"sha": "0123456789abcdef0123456789abcdef01234567"
						}`))
					}))

				client := &http.Client{
					Transport: &rewriteTransport{server: server},
				}
				resolver = gitref.NewGitHubResolverWithClient(client)
			})

			It("should resolve HTTPS URL with .git suffix", func() {
				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck.git",
					"main",
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should resolve HTTPS URL without .git suffix", func() {
				_, _, err := resolver.Resolve(
					ctx,
					"https://github.com/NVIDIA/holodeck",
					"main",
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should resolve SSH URL", func() {
				_, _, err := resolver.Resolve(
					ctx,
					"git@github.com:NVIDIA/holodeck.git",
					"main",
				)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should resolve short form URL", func() {
				_, _, err := resolver.Resolve(
					ctx,
					"github.com/NVIDIA/holodeck",
					"main",
				)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

// rewriteTransport redirects all requests to the test server.
type rewriteTransport struct {
	server *httptest.Server
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to our test server
	req.URL.Scheme = "http"
	req.URL.Host = t.server.Listener.Addr().String()
	return http.DefaultTransport.RoundTrip(req)
}

// failingTransport always returns an error.
type failingTransport struct{}

func (t *failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &networkError{message: "connection refused"}
}

type networkError struct {
	message string
}

func (e *networkError) Error() string {
	return e.message
}
