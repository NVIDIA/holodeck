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

package mocks

import (
	"net/http"
	"net/http/httptest"
)

// HTTPServer creates a mock HTTP server that returns the specified response.
// Returns the server and a cleanup function.
func HTTPServer(statusCode int, body string) (*httptest.Server, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(body)) // nolint:errcheck
	}))
	return server, server.Close
}

// HTTPServerWithHandler creates a mock HTTP server with a custom handler.
// Returns the server and a cleanup function.
func HTTPServerWithHandler(handler http.HandlerFunc) (*httptest.Server, func()) {
	server := httptest.NewServer(handler)
	return server, server.Close
}

// IPServiceServer creates a mock IP lookup service server.
func IPServiceServer(ip string) (*httptest.Server, func()) {
	return HTTPServer(http.StatusOK, ip)
}

// FailingHTTPServer creates a mock HTTP server that always returns an error.
func FailingHTTPServer() (*httptest.Server, func()) {
	return HTTPServer(http.StatusInternalServerError, "internal error")
}

// GitHubJobsServer creates a mock GitHub jobs API server.
func GitHubJobsServer(jobsJSON string) (*httptest.Server, func()) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(jobsJSON)) // nolint:errcheck
	}))
	return server, server.Close
}

// GitHubJobsCompletedResponse returns a JSON response for completed jobs.
func GitHubJobsCompletedResponse() string {
	return `{
		"jobs": [
			{"status": "completed"},
			{"status": "completed"}
		]
	}`
}

// GitHubJobsInProgressResponse returns a JSON response with in-progress jobs.
func GitHubJobsInProgressResponse() string {
	return `{
		"jobs": [
			{"status": "completed"},
			{"status": "in_progress"}
		]
	}`
}
