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
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattn/go-isatty"
)

// fdWriter is the subset of file.File that implements io.Writer and Fd()
type fdWriter interface {
	io.Writer
	Fd() uintptr
}

// Verbosity represents the logging verbosity level.
type Verbosity int

const (
	// VerbosityQuiet suppresses all output except errors.
	VerbosityQuiet Verbosity = iota
	// VerbosityNormal is the default verbosity level.
	VerbosityNormal
	// VerbosityVerbose enables debug output.
	VerbosityVerbose
	// VerbosityDebug enables trace output.
	VerbosityDebug
)

const (
	// ANSI escape code to reset color
	reset = "\033[0m"
	// ANSI escape code for green color
	green = "\033[32m"
	// ANSI escape code for yellow text
	yellowText = "\033[33m"
	// ANSI escape code for red text
	redText = "\033[31m"
	// Unicode code point for the checkmark
	checkmark = "\u2714"
	// Unicode character for the red X emoji
	redXEmoji = "\u274C"
	// Unicode character for the warning sign
	warningSign = "\u26A0"
	// Unicode character for the loading emoji
	loadingEmoji = "\U0001f300"
)

// NewLogger creates a new instance of FunLogger.
func NewLogger() *FunLogger {
	l := &FunLogger{
		Out:      os.Stderr,
		Done:     make(chan struct{}),
		Fail:     make(chan struct{}),
		Wg:       &sync.WaitGroup{},
		ExitFunc: os.Exit,
	}
	l.verbosity.Store(int32(VerbosityNormal))
	return l
}

// Printer interface defines methods for logging info, warning, and error messages.
type Logger interface {
	Info(format string, a ...any)
	Check(format string, a ...any)
	Warning(format string, a ...any)
	Error(err error)
	Loading(loadingMessage string)
	Debug(format string, a ...any)
	Trace(format string, a ...any)
	SetVerbosity(v Verbosity)
}

// FunFonts implements the Logger interface using emojis for messages.
type FunLogger struct {
	// The logs are `io.Copy`'d to this in a mutex. It's common to set this to a
	// file, or leave it default which is `os.Stderr`. You can also set this to
	// something more adventurous, such as logging to Kafka.
	Out io.Writer
	// Function to exit the application, defaults to `os.Exit()`
	ExitFunc exitFunc
	// Done is a channel that can be used to stop the loading animation.
	Done chan struct{}
	// Fail is a channel that can be used to stop the loading animation and print a failure message.
	Fail chan struct{}
	// Wg is a WaitGroup that can be used to wait for the loading animation to finish.
	Wg *sync.WaitGroup
	// IsCI is a boolean that is set to true if the logger is running in a CI environment.
	IsCI bool
	// verbosity controls the logging verbosity level (atomic for thread safety).
	verbosity atomic.Int32
}

// SetVerbosity sets the verbosity level for the logger.
func (l *FunLogger) SetVerbosity(v Verbosity) {
	l.verbosity.Store(int32(v)) //nolint:gosec // Verbosity is an iota (0-3), cannot overflow int32
}

// getVerbosity returns the current verbosity level.
func (l *FunLogger) getVerbosity() Verbosity {
	return Verbosity(l.verbosity.Load())
}

// Info prints an information message with no emoji.
// Only prints if Verbosity >= VerbosityNormal.
func (l *FunLogger) Info(format string, a ...any) {
	if l.getVerbosity() < VerbosityNormal {
		return
	}
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format += "\n"
	}

	fmt.Fprintf(l.Out, format, a...) // nolint: errcheck
}

// Check prints an information message with a check emoji.
// Only prints if Verbosity >= VerbosityNormal.
func (l *FunLogger) Check(format string, a ...any) {
	if l.getVerbosity() < VerbosityNormal {
		return
	}
	message := fmt.Sprintf(format, a...)
	printMessage(green, checkmark, message)
}

// Warning prints a warning message with a warning emoji.
// Always prints regardless of verbosity level (like Error).
func (l *FunLogger) Warning(format string, a ...any) {
	message := fmt.Sprintf(format, a...)
	printMessage(yellowText, warningSign, message)
}

// Error prints an error message with an X emoji.
// Always prints regardless of verbosity level.
func (l *FunLogger) Error(err error) {
	printMessage(redText, redXEmoji, err.Error())
}

// Debug prints a debug message.
// Only prints if Verbosity >= VerbosityVerbose.
func (l *FunLogger) Debug(format string, a ...any) {
	if l.getVerbosity() < VerbosityVerbose {
		return
	}
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(l.Out, "[DEBUG] "+format, a...) // nolint: errcheck
}

// Trace prints a trace message.
// Only prints if Verbosity >= VerbosityDebug.
func (l *FunLogger) Trace(format string, a ...any) {
	if l.getVerbosity() < VerbosityDebug {
		return
	}
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(l.Out, "[TRACE] "+format, a...) // nolint: errcheck
}

// printMessage is a helper function to print the message with the specified emoji.
func printMessage(color, emoji, message string) {
	fmt.Printf("%s%s%s\t%s\n", color, emoji, reset, message)
}

func (l *FunLogger) Loading(format string, a ...any) {
	defer l.Wg.Done()
	message := fmt.Sprintf(format, a...)
	// if running in a non-interactive terminal, don't print the loading animation
	if !l.isInteractiveTerminal() {
		// print the message with loading emoji
		printMessage(yellowText, loadingEmoji, message)
		for {
			select {
			case <-l.Done:
				return
			case <-l.Fail:
				return
			}
		}
	}

	// if message ends with a newline, remove it
	if len(message) > 0 && message[len(message)-1] == '\n' {
		message = message[:len(message)-1]
	}

	ticker := time.After(330 * time.Millisecond)
	i := 0

	spinners := []string{"|", "/", "-", "\\"}

	for {
		select {
		case <-l.Done:
			fmt.Print("\r\033[2K")
			printMessage(green, checkmark, message)
			return
		case <-l.Fail:
			fmt.Print("\r\033[2K")
			printMessage(redText, redXEmoji, message)
			return
		case <-ticker:
			i++
			fmt.Printf("\r%s\t%s", spinners[i], message)
			if i >= len(spinners)-1 {
				i = 0
			}

			ticker = time.After(330 * time.Millisecond)
		}
	}
}

func (l *FunLogger) isInteractiveTerminal() bool {
	return isTerminal(os.Stdout) && !l.isCILogs()
}

func (l *FunLogger) isCILogs() bool {
	if os.Getenv("CI") == "true" {
		return true
	}
	return l.IsCI
}

func (l *FunLogger) Exit(code int) {
	// Stop the loading animation if it's running
	close(l.Done)
	l.Wg.Wait()

	l.ExitFunc(code)
}

// isTerminal returns whether we have a terminal or not
func isTerminal(w fdWriter) bool {
	return isatty.IsTerminal(w.Fd())
}

type exitFunc func(int)
