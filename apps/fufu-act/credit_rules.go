package activityapp

const (
	creditStatusPending = "pending"
	creditStatusFailed  = "failed"
	creditStatusDone    = "done"
)

type creditFailureUpdate struct {
	Retries int
	Status  string
	Error   string
}

func buildCreditFailureUpdate(currentRetries, maxRetries int, err error) creditFailureUpdate {
	nextRetries := currentRetries + 1
	status := creditStatusPending
	if maxRetries <= 0 || nextRetries >= maxRetries {
		status = creditStatusFailed
	}
	return creditFailureUpdate{
		Retries: nextRetries,
		Status:  status,
		Error:   sanitizeCreditFailureError(err),
	}
}

func sanitizeCreditScanError(error) string {
	return "队列数据异常，请人工检查"
}

func sanitizeCreditFailureError(error) string {
	return "派奖失败，请人工处理"
}
