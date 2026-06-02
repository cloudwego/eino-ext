/*
 * Copyright 2026 CloudWeGo Authors
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

package agenticgemini

type ServerToolName string

const (
	ServerToolNameCodeExecution ServerToolName = "CodeExecution"
)

type Language string

const (
	LanguageUnspecified Language = "LANGUAGE_UNSPECIFIED"
	LanguagePython      Language = "PYTHON"
)

type Outcome string

const (
	// OutcomeUnspecified specifies that unspecified status. This value should not be used.
	OutcomeUnspecified Outcome = "OUTCOME_UNSPECIFIED"
	// OutcomeOK specifies that code execution completed successfully.
	OutcomeOK Outcome = "OUTCOME_OK"
	// OutcomeFailed specifies that code execution finished but with a failure. `stderr` should contain the reason.
	OutcomeFailed Outcome = "OUTCOME_FAILED"
	// OutcomeDeadlineExceeded specifies that code execution ran for too long, and was cancelled. There may or may not be a partial
	// output present.
	OutcomeDeadlineExceeded Outcome = "OUTCOME_DEADLINE_EXCEEDED"
)
