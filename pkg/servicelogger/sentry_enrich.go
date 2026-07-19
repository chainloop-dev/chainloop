//
// Copyright 2026 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package servicelogger

import (
	"errors"
	"reflect"
	"strconv"

	"github.com/getsentry/sentry-go"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"google.golang.org/grpc/status"
)

const (
	// maxUnwrapDepth caps the error-chain walk to guard against pathological cycles
	maxUnwrapDepth = 32
	// maxChainVisits bounds total DFS visits so wide joins exhaust visits before the depth cap
	maxChainVisits = maxUnwrapDepth * 4
)

// genericWrapperTypes are concrete error types produced by wrapping helpers
// (fmt.Errorf, pkg/errors) that say nothing about the failure itself. When the
// primary Sentry exception has one of these types it is rewritten to the
// root-cause type so issue titles become meaningful.
var genericWrapperTypes = map[string]struct{}{
	"*fmt.wrapError":      {},
	"*fmt.withMessage":    {},
	"*errors.withMessage": {},
	"*errors.withStack":   {},
	"*errors.fundamental": {},
}

// errorChain summarizes an error unwrap chain for Sentry enrichment
type errorChain struct {
	rootType     string // Go type of the innermost error
	depth        int
	sqlState     string // SQLSTATE from pgx/pgconn-style errors
	grpcCode     string // gRPC status code from status errors
	kratosReason string // reason from kratos errors
	multi        bool   // chain contains a multi-error (Unwrap() []error, e.g. errors.Join)
}

// SentryBeforeSend enriches Sentry error events so issues are distinguishable
// at a glance and group deterministically by failure mode:
//   - A structured discriminator extends Sentry's default fingerprint
//   - A generic wrapper type on the primary exception is rewritten to the root-cause type
//   - Tags expose the root-cause metadata for search
//
// Events without an original error to analyze (messages, transactions) and
// errors without a structured discriminator are left
// untouched, preserving Sentry's default grouping. Joined multi-errors
// (errors.Join) also keep default grouping: bundling independent failures
// makes any single fingerprint misleading, though their metadata is tagged.
func SentryBeforeSend(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
	if event == nil || len(event.Exception) == 0 {
		return event
	}

	original := originalError(hint)
	if original == nil {
		return event
	}

	info := analyzeErrorChain(original)

	// Joined multi-errors bundle independent failures: any single root type or
	// fingerprint would arbitrarily pair attributes from different branches, so
	// they keep Sentry's default grouping and their factual wrapper type
	if !info.multi && len(event.Fingerprint) == 0 {
		if fp := fingerprint(info); fp != nil {
			event.Fingerprint = fp
		}
	}

	// sentry-go orders exceptions root-first, so the last entry is the
	// outermost (primary) exception used for the issue title
	primary := &event.Exception[len(event.Exception)-1]
	if _, generic := genericWrapperTypes[primary.Type]; generic && !info.multi {
		primary.Type = info.rootType
	}

	if event.Tags == nil {
		event.Tags = make(map[string]string)
	}
	event.Tags["error.chain_depth"] = strconv.Itoa(info.depth)
	if info.multi {
		event.Tags["error.multi_error"] = "true"
	} else {
		event.Tags["error.root_type"] = info.rootType
	}
	if info.sqlState != "" {
		event.Tags["error.sqlstate"] = info.sqlState
	}
	if info.grpcCode != "" {
		event.Tags["error.grpc_code"] = info.grpcCode
	}
	if info.kratosReason != "" {
		event.Tags["error.kratos_reason"] = info.kratosReason
	}

	return event
}

// originalError extracts the captured error from the hint, covering both
// CaptureException and recovered panics
func originalError(hint *sentry.EventHint) error {
	if hint == nil {
		return nil
	}

	if hint.OriginalException != nil {
		return hint.OriginalException
	}

	if recovered, ok := hint.RecoveredException.(error); ok {
		return recovered
	}

	return nil
}

// analyzeErrorChain walks err's unwrap chain collecting the root-cause type,
// structured discriminators (SQLSTATE, gRPC code, kratos reason). It traverses
// both single (Unwrap() error) and
// multi (Unwrap() []error, e.g. errors.Join) branches like sentry-go does.
// The walk is bounded by depth and a total-visit budget so cyclic chains and
// wide joins terminate. Discriminators are collected via type assertions
// instead of errors.As because the stdlib traversal has no cycle detection.
func analyzeErrorChain(err error) errorChain {
	var info errorChain
	if err == nil {
		return info
	}

	budget := maxChainVisits
	deepest := err
	var visit func(cur error, depth int)
	visit = func(cur error, depth int) {
		if cur == nil || depth > maxUnwrapDepth || budget <= 0 {
			return
		}
		budget--

		if depth > info.depth {
			info.depth = depth
			deepest = cur
		}

		if se, ok := cur.(interface{ SQLState() string }); ok && info.sqlState == "" {
			info.sqlState = se.SQLState()
		}
		if ge, ok := cur.(interface{ GRPCStatus() *status.Status }); ok && info.grpcCode == "" {
			// nil-check: third-party implementations may return a nil status
			if s := ge.GRPCStatus(); s != nil {
				info.grpcCode = s.Code().String()
			}
		}
		if ke, ok := cur.(*kerrors.Error); ok && info.kratosReason == "" { //nolint:errorlint // per-node assertion by design: errors.As has no cycle protection
			info.kratosReason = ke.Reason
		}

		if multi, ok := cur.(interface{ Unwrap() []error }); ok {
			info.multi = true
			for _, child := range multi.Unwrap() {
				visit(child, depth+1)
			}
			return
		}
		visit(errors.Unwrap(cur), depth+1)
	}
	visit(err, 0)

	info.rootType = reflect.TypeOf(deepest).String()
	return info
}

// fingerprint extends Sentry's default grouping with a structured discriminator.
// `{{ default }}` keeps Sentry's grouping heuristics (stacktrace, message) and
// adds a SQLSTATE/gRPC/kratos subdivision. Call-site separation is not
// guaranteed: AttachStacktrace captures the shared LogAndMaskErr frame, so the
// type rewrite and default message heuristics are what actually distinguish
// events. Formatted error text is deliberately excluded from the fingerprint
// because it may contain request-specific values that cause cardinality.
func fingerprint(info errorChain) []string {
	discriminator := info.sqlState
	if discriminator == "" {
		discriminator = info.grpcCode
	}
	if discriminator == "" {
		discriminator = info.kratosReason
	}

	if discriminator != "" {
		return []string{"{{ default }}", discriminator}
	}

	return nil
}
