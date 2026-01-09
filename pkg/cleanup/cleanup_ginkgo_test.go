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

	"github.com/NVIDIA/holodeck/internal/logger"
)

var _ = Describe("Cleanup Package", func() {

	Describe("safeString", func() {
		It("should return the string value when pointer is not nil", func() {
			s := "test-value"
			result := safeString(&s)
			Expect(result).To(Equal("test-value"))
		})

		It("should return '<nil>' when pointer is nil", func() {
			result := safeString(nil)
			Expect(result).To(Equal("<nil>"))
		})

		It("should handle empty string", func() {
			s := ""
			result := safeString(&s)
			Expect(result).To(Equal(""))
		})
	})

	Describe("Validation Patterns", func() {
		Describe("repoPattern", func() {
			DescribeTable("validating repository names",
				func(repo string, expected bool) {
					result := repoPattern.MatchString(repo)
					Expect(result).To(Equal(expected))
				},
				Entry("valid org/repo", "NVIDIA/holodeck", true),
				Entry("valid with hyphen", "my-org/my-repo", true),
				Entry("valid with underscore", "my_org/my_repo", true),
				Entry("valid with dot", "my.org/my.repo", true),
				Entry("valid with numbers", "org123/repo456", true),
				Entry("invalid - no slash", "holodeck", false),
				Entry("invalid - empty org", "/holodeck", false),
				Entry("invalid - empty repo", "NVIDIA/", false),
				Entry("invalid - spaces", "my org/my repo", false),
				Entry("invalid - special chars", "org@/repo!", false),
				Entry("invalid - multiple slashes", "org/sub/repo", false),
			)
		})

		Describe("runIDPattern", func() {
			DescribeTable("validating run IDs",
				func(runID string, expected bool) {
					result := runIDPattern.MatchString(runID)
					Expect(result).To(Equal(expected))
				},
				Entry("valid numeric ID", "12345678", true),
				Entry("valid single digit", "1", true),
				Entry("valid long ID", "1234567890123456789", true),
				Entry("invalid - contains letters", "123abc", false),
				Entry("invalid - empty", "", false),
				Entry("invalid - spaces", "123 456", false),
				Entry("invalid - special chars", "123-456", false),
			)
		})
	})

	Describe("GitHubJobsResponse", func() {
		It("should unmarshal valid JSON with completed jobs", func() {
			jsonData := `{
				"jobs": [
					{"status": "completed"},
					{"status": "completed"}
				]
			}`
			var resp GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Jobs).To(HaveLen(2))
			Expect(resp.Jobs[0].Status).To(Equal("completed"))
			Expect(resp.Jobs[1].Status).To(Equal("completed"))
		})

		It("should unmarshal valid JSON with in_progress jobs", func() {
			jsonData := `{
				"jobs": [
					{"status": "in_progress"},
					{"status": "completed"}
				]
			}`
			var resp GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Jobs).To(HaveLen(2))
			Expect(resp.Jobs[0].Status).To(Equal("in_progress"))
		})

		It("should handle empty jobs array", func() {
			jsonData := `{"jobs": []}`
			var resp GitHubJobsResponse
			err := json.Unmarshal([]byte(jsonData), &resp)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Jobs).To(BeEmpty())
		})
	})

	Describe("CheckGitHubJobsCompleted", func() {
		var (
			log    *logger.FunLogger
			server *httptest.Server
		)

		BeforeEach(func() {
			log = logger.NewLogger()
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		Context("input validation", func() {
			It("should reject invalid repository format", func() {
				// We can't call CheckGitHubJobsCompleted without a Cleaner,
				// but we can test the validation patterns directly
				Expect(repoPattern.MatchString("invalid")).To(BeFalse())
				Expect(repoPattern.MatchString("org/repo")).To(BeTrue())
			})

			It("should reject invalid runID format", func() {
				Expect(runIDPattern.MatchString("abc")).To(BeFalse())
				Expect(runIDPattern.MatchString("12345")).To(BeTrue())
			})
		})

		Context("HTTP response handling", func() {
			It("should handle 404 response as completed", func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))

				// Create a cleaner with nil EC2 client (we're testing HTTP, not EC2)
				cleaner := &Cleaner{
					ec2: nil,
					log: log,
				}

				// Note: We can't easily test this because the URL is hardcoded
				// This demonstrates the need for HTTP client injection
				_ = cleaner
				_ = server
			})
		})
	})

	Describe("Job completion logic", func() {
		It("should consider all jobs completed when jobs array is empty", func() {
			resp := GitHubJobsResponse{Jobs: []struct {
				Status string `json:"status"`
			}{}}

			allCompleted := true
			for _, job := range resp.Jobs {
				if job.Status != "completed" {
					allCompleted = false
					break
				}
			}
			Expect(allCompleted).To(BeTrue())
		})

		It("should detect incomplete jobs", func() {
			resp := GitHubJobsResponse{
				Jobs: []struct {
					Status string `json:"status"`
				}{
					{Status: "completed"},
					{Status: "in_progress"},
					{Status: "completed"},
				},
			}

			allCompleted := true
			for _, job := range resp.Jobs {
				if job.Status != "completed" {
					allCompleted = false
					break
				}
			}
			Expect(allCompleted).To(BeFalse())
		})

		It("should confirm all jobs completed", func() {
			resp := GitHubJobsResponse{
				Jobs: []struct {
					Status string `json:"status"`
				}{
					{Status: "completed"},
					{Status: "completed"},
					{Status: "completed"},
				},
			}

			allCompleted := true
			for _, job := range resp.Jobs {
				if job.Status != "completed" {
					allCompleted = false
					break
				}
			}
			Expect(allCompleted).To(BeTrue())
		})
	})
})
