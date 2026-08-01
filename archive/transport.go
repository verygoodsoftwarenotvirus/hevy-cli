package archive

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/verygoodsoftwarenotvirus/hevy-cli"
)

const (
	defaultAttempts    = 5
	defaultBaseBackoff = time.Second
	maxBackoff         = 30 * time.Second
	// drainLimit caps how much of a discarded response body is read back to allow connection reuse.
	drainLimit = 16 << 10
)

// NewClient builds a Hevy client that retries rate-limited and transient failures.
//
// A year of history is dozens to hundreds of sequential requests at the API's fixed page size, so a
// single 429 partway through would otherwise abort the whole archive.
func NewClient(apiKey string) *hevy.Client {
	return hevy.NewClient(apiKey, hevy.WithHTTPClient(&http.Client{
		// No overall client deadline: the per-attempt timeouts below bound each request and the
		// caller's context bounds the run. A client timeout here would race the backoff.
		Transport: &retryTransport{
			base: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
			},
			attempts:    defaultAttempts,
			baseBackoff: defaultBaseBackoff,
		},
	}))
}

// retryTransport retries 429 and 5xx responses, and transport errors, with exponential backoff.
type retryTransport struct {
	base        http.RoundTripper
	attempts    int
	baseBackoff time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// A request whose body cannot be replayed must not be retried.
	replayable := req.Body == nil || req.GetBody != nil

	for attempt := 1; ; attempt++ {
		resp, err := t.base.RoundTrip(req)

		if err == nil && !retryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// Out of attempts: hand back whatever we got, including a retryable status, and let the
		// client turn it into an APIError.
		if attempt >= t.attempts || !replayable {
			return resp, err
		}

		// A cancelled or expired context is the caller's decision, never ours to retry past.
		if ctx.Err() != nil {
			return resp, err
		}

		var hint time.Duration
		if err == nil {
			hint = retryAfter(resp)
			drain(resp)
		}

		if sleepErr := sleep(ctx, t.backoff(attempt, hint)); sleepErr != nil {
			return nil, sleepErr
		}
		if rewindErr := rewind(req); rewindErr != nil {
			return nil, rewindErr
		}
	}
}

// backoff returns how long to wait before the next attempt, honoring a server hint whenever it is
// longer than our own schedule.
func (t *retryTransport) backoff(attempt int, hint time.Duration) time.Duration {
	wait := t.baseBackoff << (attempt - 1)
	if wait <= 0 || wait > maxBackoff {
		wait = maxBackoff
	}
	if hint > wait {
		wait = hint
	}
	return wait
}

func retryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= http.StatusInternalServerError
}

// retryAfter reads the Retry-After header, which may be a count of seconds or an HTTP date.
func retryAfter(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}

	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}

	if when, err := http.ParseTime(v); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}

	return 0
}

// drain consumes and closes a response body being discarded so the connection can be reused.
// Failures here cost nothing but that reuse, so they are logged rather than returned.
func drain(resp *http.Response) {
	if resp.Body == nil {
		return
	}

	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimit)); err != nil {
		slog.Debug("draining discarded response body", "error", err)
	}
	if err := resp.Body.Close(); err != nil {
		slog.Debug("closing discarded response body", "error", err)
	}
}

// rewind resets a replayable request body before the next attempt.
func rewind(req *http.Request) error {
	if req.Body == nil || req.GetBody == nil {
		return nil
	}

	body, err := req.GetBody()
	if err != nil {
		return err
	}
	req.Body = body

	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
