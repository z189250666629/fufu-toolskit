package activityapp

import (
	"errors"
	"testing"
)

func TestCreditFailureUpdateRetriesUntilLimit(t *testing.T) {
	update := buildCreditFailureUpdate(2, 5, errors.New("raw upstream secret"))

	if update.Retries != 3 {
		t.Fatalf("retries=%d, want 3", update.Retries)
	}
	if update.Status != creditStatusPending {
		t.Fatalf("status=%q, want %q", update.Status, creditStatusPending)
	}
	if update.Error != "派奖失败，请人工处理" {
		t.Fatalf("error=%q", update.Error)
	}
}

func TestCreditFailureUpdateFailsAtRetryLimit(t *testing.T) {
	update := buildCreditFailureUpdate(4, 5, errors.New("raw upstream secret"))

	if update.Retries != 5 {
		t.Fatalf("retries=%d, want 5", update.Retries)
	}
	if update.Status != creditStatusFailed {
		t.Fatalf("status=%q, want %q", update.Status, creditStatusFailed)
	}
}

func TestCreditFailureUpdateHandlesInvalidRetryLimit(t *testing.T) {
	update := buildCreditFailureUpdate(0, 0, errors.New("raw upstream secret"))

	if update.Retries != 1 {
		t.Fatalf("retries=%d, want 1", update.Retries)
	}
	if update.Status != creditStatusFailed {
		t.Fatalf("status=%q, want %q", update.Status, creditStatusFailed)
	}
}

func TestSanitizeCreditErrorsUseStableOperatorMessages(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{name: "scan", got: sanitizeCreditScanError(errors.New("scan raw card key")), want: "队列数据异常，请人工检查"},
		{name: "failure", got: sanitizeCreditFailureError(errors.New("https://newapi.example secret")), want: "派奖失败，请人工处理"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("message=%q, want %q", tc.got, tc.want)
			}
		})
	}
}
