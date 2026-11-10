package twitter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/oaooao/webx/internal/types"
)

func stubSleep(t *testing.T) *[]time.Duration {
	t.Helper()
	old := sleep
	var slept []time.Duration
	sleep = func(d time.Duration) { slept = append(slept, d) }
	t.Cleanup(func() { sleep = old })
	return &slept
}

func TestWithRetry_TransientThenSuccess(t *testing.T) {
	slept := stubSleep(t)
	calls := 0
	raw, err := withRetry(func() (json.RawMessage, error, bool) {
		calls++
		if calls == 1 {
			return nil, types.NewWebxError(types.ErrFetchFailed, "connection reset by peer"), true
		}
		return json.RawMessage(`{}`), nil, false
	})
	if err != nil || string(raw) != "{}" {
		t.Fatalf("raw=%s err=%v", raw, err)
	}
	if calls != 2 || len(*slept) != 1 {
		t.Errorf("calls=%d sleeps=%v", calls, *slept)
	}
}

func TestWithRetry_GivesUpAfterDelaysAndSaysSo(t *testing.T) {
	slept := stubSleep(t)
	calls := 0
	_, err := withRetry(func() (json.RawMessage, error, bool) {
		calls++
		return nil, types.NewWebxError(types.ErrFetchFailed, "connection reset by peer"), true
	})
	if calls != len(retryDelays)+1 || len(*slept) != len(retryDelays) {
		t.Errorf("calls=%d sleeps=%v", calls, *slept)
	}
	if err == nil || !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("err = %v", err)
	}
}

func TestWithRetry_NonTransientIsNotRetried(t *testing.T) {
	stubSleep(t)
	calls := 0
	_, err := withRetry(func() (json.RawMessage, error, bool) {
		calls++
		return nil, types.NewWebxError(types.ErrRateLimited, "429"), false
	})
	if calls != 1 || err == nil || strings.Contains(err.Error(), "attempts") {
		t.Errorf("calls=%d err=%v", calls, err)
	}
}
