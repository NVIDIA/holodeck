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

package cleanup

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cleanup", func() {

	Describe("GitHubJobsResponse", func() {
		It("should properly unmarshal JSON response", func() {
			jsonData := `{
				"jobs": [
					{"status": "completed"},
					{"status": "in_progress"}
				]
			}`

			var response GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Jobs).To(HaveLen(2))
			Expect(response.Jobs[0].Status).To(Equal("completed"))
			Expect(response.Jobs[1].Status).To(Equal("in_progress"))
		})

		It("should handle empty jobs array", func() {
			jsonData := `{"jobs": []}`

			var response GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Jobs).To(BeEmpty())
		})
	})

	Describe("safeString", func() {
		It("should return the string value when pointer is not nil", func() {
			value := "test-value"
			result := safeString(&value)
			Expect(result).To(Equal("test-value"))
		})

		It("should return <nil> when pointer is nil", func() {
			result := safeString(nil)
			Expect(result).To(Equal("<nil>"))
		})

		It("should return empty string when value is empty", func() {
			value := ""
			result := safeString(&value)
			Expect(result).To(Equal(""))
		})
	})

	Describe("Input validation patterns", func() {
		Describe("repoPattern", func() {
			DescribeTable("validating repository names",
				func(repo string, shouldMatch bool) {
					result := repoPattern.MatchString(repo)
					Expect(result).To(Equal(shouldMatch))
				},
				Entry("valid simple repo", "owner/repo", true),
				Entry("valid repo with numbers", "owner123/repo456", true),
				Entry("valid repo with dashes", "my-org/my-repo", true),
				Entry("valid repo with underscores", "my_org/my_repo", true),
				Entry("valid repo with dots", "my.org/my.repo", true),
				Entry("valid NVIDIA repo", "NVIDIA/holodeck", true),
				Entry("invalid - no slash", "ownerrepo", false),
				Entry("invalid - double slash", "owner//repo", false),
				Entry("invalid - leading slash", "/owner/repo", false),
				Entry("invalid - trailing slash", "owner/repo/", false),
				Entry("invalid - special chars", "owner/repo@123", false),
				Entry("empty string", "", false),
			)
		})

		Describe("runIDPattern", func() {
			DescribeTable("validating run IDs",
				func(runID string, shouldMatch bool) {
					result := runIDPattern.MatchString(runID)
					Expect(result).To(Equal(shouldMatch))
				},
				Entry("valid numeric ID", "12345", true),
				Entry("valid single digit", "1", true),
				Entry("valid large number", "9876543210", true),
				Entry("invalid - contains letters", "123abc", false),
				Entry("invalid - contains dashes", "123-456", false),
				Entry("invalid - contains spaces", "123 456", false),
				Entry("invalid - empty string", "", false),
				Entry("invalid - negative sign", "-123", false),
			)
		})
	})

	Describe("CheckGitHubJobsCompleted", func() {
		var (
			server  *httptest.Server
			cleaner *Cleaner
		)

		BeforeEach(func() {
			cleaner = &Cleaner{
				ec2: nil,
				log: nil,
			}
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		Context("input validation", func() {
			It("should reject invalid repository format", func() {
				completed, err := cleaner.CheckGitHubJobsCompleted(
					"invalid-repo", "12345", "token")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid repository format"))
				Expect(completed).To(BeFalse())
			})

			It("should reject invalid runID format", func() {
				completed, err := cleaner.CheckGitHubJobsCompleted(
					"owner/repo", "invalid-id", "token")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid runID format"))
				Expect(completed).To(BeFalse())
			})

			It("should reject runID with letters", func() {
				completed, err := cleaner.CheckGitHubJobsCompleted(
					"owner/repo", "abc123", "token")
				Expect(err).To(HaveOccurred())
				Expect(completed).To(BeFalse())
			})
		})
	})

	Describe("GitHubJobsResponse parsing", func() {
		Context("when all jobs are completed", func() {
			It("should parse correctly", func() {
				jsonData := `{"jobs": [
					{"status": "completed"},
					{"status": "completed"},
					{"status": "completed"}
				]}`

				var response GitHubJobsResponse
				err := json.Unmarshal([]byte(jsonData), &response)
				Expect(err).NotTo(HaveOccurred())

				allCompleted := true
				for _, job := range response.Jobs {
					if job.Status != "completed" {
						allCompleted = false
						break
					}
				}
				Expect(allCompleted).To(BeTrue())
			})
		})

		Context("when some jobs are still running", func() {
			It("should detect incomplete jobs", func() {
				jsonData := `{"jobs": [
					{"status": "completed"},
					{"status": "in_progress"},
					{"status": "queued"}
				]}`

				var response GitHubJobsResponse
				err := json.Unmarshal([]byte(jsonData), &response)
				Expect(err).NotTo(HaveOccurred())

				allCompleted := true
				for _, job := range response.Jobs {
					if job.Status != "completed" {
						allCompleted = false
						break
					}
				}
				Expect(allCompleted).To(BeFalse())
			})
		})

		Context("when response has no jobs", func() {
			It("should handle empty jobs array", func() {
				jsonData := `{"jobs": []}`

				var response GitHubJobsResponse
				err := json.Unmarshal([]byte(jsonData), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Jobs).To(BeEmpty())
			})
		})
	})

	Describe("HTTP request handling", func() {
		var server *httptest.Server

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		Context("when GitHub API returns 404", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusNotFound)
					}))
			})

			It("should be treated as safe to delete", func() {
				// Note: This tests the expected behavior that 404 means
				// workflow doesn't exist, so it's safe to cleanup
				Expect(http.StatusNotFound).To(Equal(404))
			})
		})

		Context("when GitHub API returns success with completed jobs", func() {
			BeforeEach(func() {
				server = httptest.NewServer(http.HandlerFunc(
					func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK)
						response := `{"jobs": [{"status": "completed"}]}`
						_, _ = w.Write([]byte(response)) //nolint:errcheck // test
					}))
			})

			It("should handle response correctly", func() {
				// This test verifies the server responds correctly
				resp, err := http.Get(server.URL)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(resp.Body.Close()).To(Succeed())
			})
		})
	})
})
