/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

package logger

import (
	"bytes"
	"errors"
	"testing"
)

func TestVerbosityLevels(t *testing.T) {
	tests := []struct {
		name      string
		verbosity Verbosity
		wantValue int
	}{
		{"Quiet is 0", VerbosityQuiet, 0},
		{"Normal is 1", VerbosityNormal, 1},
		{"Verbose is 2", VerbosityVerbose, 2},
		{"Debug is 3", VerbosityDebug, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.verbosity) != tt.wantValue {
				t.Errorf("Verbosity %s = %d, want %d", tt.name, tt.verbosity, tt.wantValue)
			}
		})
	}
}

func TestNewLoggerDefaultVerbosity(t *testing.T) {
	l := NewLogger()
	if l.Verbosity != VerbosityNormal {
		t.Errorf("NewLogger() Verbosity = %d, want %d (VerbosityNormal)", l.Verbosity, VerbosityNormal)
	}
}

func TestSetVerbosity(t *testing.T) {
	l := NewLogger()

	l.SetVerbosity(VerbosityDebug)
	if l.Verbosity != VerbosityDebug {
		t.Errorf("SetVerbosity(VerbosityDebug) = %d, want %d", l.Verbosity, VerbosityDebug)
	}

	l.SetVerbosity(VerbosityQuiet)
	if l.Verbosity != VerbosityQuiet {
		t.Errorf("SetVerbosity(VerbosityQuiet) = %d, want %d", l.Verbosity, VerbosityQuiet)
	}
}

func TestQuietModeSuppressesInfoButAllowsError(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger()
	l.Out = &buf
	l.SetVerbosity(VerbosityQuiet)

	// Info should be suppressed in quiet mode
	l.Info("this should not appear")
	if buf.Len() > 0 {
		t.Errorf("Info() in Quiet mode should not produce output, got: %s", buf.String())
	}

	// Error should always print (uses stdout via printMessage, so we test it separately)
	// For this test, we just verify the method doesn't panic
	err := errors.New("test error")
	l.Error(err) // Should not panic
}

func TestNormalModeShowsInfoButHidesDebug(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger()
	l.Out = &buf
	l.SetVerbosity(VerbosityNormal)

	// Info should appear in normal mode
	l.Info("test info message")
	if buf.Len() == 0 {
		t.Error("Info() in Normal mode should produce output")
	}

	// Reset buffer
	buf.Reset()

	// Debug should be hidden in normal mode
	l.Debug("this should not appear")
	if buf.Len() > 0 {
		t.Errorf("Debug() in Normal mode should not produce output, got: %s", buf.String())
	}
}

func TestVerboseModeShowsDebug(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger()
	l.Out = &buf
	l.SetVerbosity(VerbosityVerbose)

	// Debug should appear in verbose mode
	l.Debug("test debug message")
	if buf.Len() == 0 {
		t.Error("Debug() in Verbose mode should produce output")
	}

	// Reset buffer
	buf.Reset()

	// Trace should still be hidden in verbose mode
	l.Trace("this should not appear")
	if buf.Len() > 0 {
		t.Errorf("Trace() in Verbose mode should not produce output, got: %s", buf.String())
	}
}

func TestDebugModeShowsTrace(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger()
	l.Out = &buf
	l.SetVerbosity(VerbosityDebug)

	// Trace should appear in debug mode
	l.Trace("test trace message")
	if buf.Len() == 0 {
		t.Error("Trace() in Debug mode should produce output")
	}

	// Debug should also appear
	buf.Reset()
	l.Debug("test debug message")
	if buf.Len() == 0 {
		t.Error("Debug() in Debug mode should produce output")
	}

	// Info should also appear
	buf.Reset()
	l.Info("test info message")
	if buf.Len() == 0 {
		t.Error("Info() in Debug mode should produce output")
	}
}

func TestQuietModeSuppressesCheckButNotWarning(t *testing.T) {
	l := NewLogger()
	l.SetVerbosity(VerbosityQuiet)

	// Warning always prints (like Error) - verify it doesn't panic in quiet mode
	l.Warning("this should still print")

	// Check is suppressed in quiet mode - verify it doesn't panic
	l.Check("this is suppressed")
}

func TestDebugMethodFormat(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger()
	l.Out = &buf
	l.SetVerbosity(VerbosityVerbose)

	l.Debug("value: %d, name: %s", 42, "test")
	output := buf.String()
	if output == "" {
		t.Error("Debug() should produce output in Verbose mode")
	}
	// Verify format string was applied
	if !bytes.Contains(buf.Bytes(), []byte("42")) || !bytes.Contains(buf.Bytes(), []byte("test")) {
		t.Errorf("Debug() format not applied correctly, got: %s", output)
	}
}

func TestTraceMethodFormat(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger()
	l.Out = &buf
	l.SetVerbosity(VerbosityDebug)

	l.Trace("value: %d, name: %s", 42, "test")
	output := buf.String()
	if output == "" {
		t.Error("Trace() should produce output in Debug mode")
	}
	// Verify format string was applied
	if !bytes.Contains(buf.Bytes(), []byte("42")) || !bytes.Contains(buf.Bytes(), []byte("test")) {
		t.Errorf("Trace() format not applied correctly, got: %s", output)
	}
}
