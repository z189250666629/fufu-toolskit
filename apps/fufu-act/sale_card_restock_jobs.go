package activityapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"fufu/tokens"
)

const (
	saleCardRestockStatusPending   = "pending"
	saleCardRestockStatusRunning   = "running"
	saleCardRestockStatusSucceeded = "succeeded"
	saleCardRestockStatusFailed    = "failed"
)

var (
	saleCardRestockVerifyTimeout    = 45 * time.Second
	saleCardRestockLockGrace        = 30 * time.Second
	saleCardRestockRetryDelay       = 30 * time.Second
	saleCardRestockMaxTimeouts      = 3
	saleCardRestockMaxJobsPerPass   = 20
	saleCardRestockStatusReportSize = 30
)

type SaleCardRestockJobStatus struct {
	ID                  int64  `json:"id"`
	BizDate             string `json:"bizDate"`
	SlotGroup           string `json:"slotGroup"`
	SlotTime            string `json:"slotTime"`
	PlanID              string `json:"planId"`
	TargetStock         int    `json:"targetStock"`
	Status              string `json:"status"`
	Attempts            int    `json:"attempts"`
	ConsecutiveTimeouts int    `json:"consecutiveTimeouts"`
	CurrentStock        int    `json:"currentStock"`
	Uploaded            int    `json:"uploaded"`
	LastError           string `json:"lastError,omitempty"`
	FailureReason       string `json:"failureReason,omitempty"`
	UpdatedAt           string `json:"updatedAt,omitempty"`
	FinishedAt          string `json:"finishedAt,omitempty"`
}

type SaleCardRestockStatus struct {
	Jobs []SaleCardRestockJobStatus `json:"jobs"`
}

type saleCardRestockJob struct {
	ID                  int64
	BizDate             string
	SlotGroup           string
	SlotTime            string
	PlanID              string
	TargetStock         int
	Status              string
	Attempts            int
	ConsecutiveTimeouts int
	CurrentStock        int
	Uploaded            int
	LastError           sql.NullString
	FailureReason       sql.NullString
	RunToken            sql.NullString
	LockedUntil         int64
	UpdatedAt           sql.NullString
	FinishedAt          sql.NullString
}

func enqueueSaleCardRestockJobs(bizDate string, slot SaleCardScheduleSlot) error {
	if db == nil {
		return errors.New("sale card restock db is not configured")
	}
	for _, job := range slot.Jobs {
		if !job.Enabled || job.TargetStock <= 0 {
			continue
		}
		if _, err := db.Exec(`INSERT OR IGNORE INTO sale_card_restock_jobs
			(biz_date, slot_group, slot_time, plan_id, target_stock, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))`,
			bizDate, strings.TrimSpace(slot.Group), strings.TrimSpace(slot.Time), strings.TrimSpace(job.Plan), job.TargetStock, saleCardRestockStatusPending,
		); err != nil {
			return err
		}
	}
	return nil
}

func processSaleCardRestockJobs(now time.Time) {
	service, _ := snapshotTokenRuntime()
	if service == nil {
		fmt.Printf("[sale-card] restock worker skipped: 次数 fufu 未配置\n")
		return
	}
	for i := 0; i < saleCardRestockMaxJobsPerPass; i++ {
		job, ok, err := claimNextSaleCardRestockJob(now)
		if err != nil {
			fmt.Printf("[sale-card] claim restock job failed: %v\n", err)
			return
		}
		if !ok {
			return
		}
		runClaimedSaleCardRestockJob(job, service)
	}
}

func claimNextSaleCardRestockJob(now time.Time) (saleCardRestockJob, bool, error) {
	if db == nil {
		return saleCardRestockJob{}, false, errors.New("sale card restock db is not configured")
	}
	runToken := fmt.Sprintf("%d", now.UnixNano())
	lockedUntil := now.Add(saleCardRestockTimeout + saleCardRestockLockGrace).Unix()
	for {
		job, ok, err := selectClaimableSaleCardRestockJob(now)
		if err != nil || !ok {
			return saleCardRestockJob{}, ok, err
		}
		res, err := db.Exec(`UPDATE sale_card_restock_jobs
			SET status=?, attempts=attempts+1, run_token=?, locked_until=?, started_at=datetime('now'), updated_at=datetime('now')
			WHERE id=? AND (
				(status=? AND locked_until<=?)
				OR (status=? AND locked_until<=?)
			)`,
			saleCardRestockStatusRunning, runToken, lockedUntil, job.ID,
			saleCardRestockStatusPending, now.Unix(), saleCardRestockStatusRunning, now.Unix(),
		)
		if err != nil {
			return saleCardRestockJob{}, false, err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			continue
		}
		job.Status = saleCardRestockStatusRunning
		job.Attempts++
		job.RunToken = sql.NullString{String: runToken, Valid: true}
		job.LockedUntil = lockedUntil
		return job, true, nil
	}
}

func selectClaimableSaleCardRestockJob(now time.Time) (saleCardRestockJob, bool, error) {
	row := db.QueryRow(`SELECT id, biz_date, slot_group, slot_time, plan_id, target_stock, status,
			attempts, consecutive_timeouts, current_stock, uploaded, last_error, failure_reason,
			run_token, locked_until, updated_at, finished_at
		FROM sale_card_restock_jobs
		WHERE (status=? AND locked_until<=?)
			OR (status=? AND locked_until<=?)
		ORDER BY id ASC
		LIMIT 1`,
		saleCardRestockStatusPending, now.Unix(), saleCardRestockStatusRunning, now.Unix(),
	)
	job, err := scanSaleCardRestockJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return saleCardRestockJob{}, false, nil
	}
	if err != nil {
		return saleCardRestockJob{}, false, err
	}
	return job, true, nil
}

type saleCardRestockJobScanner interface {
	Scan(dest ...any) error
}

func scanSaleCardRestockJob(row saleCardRestockJobScanner) (saleCardRestockJob, error) {
	var job saleCardRestockJob
	err := row.Scan(
		&job.ID,
		&job.BizDate,
		&job.SlotGroup,
		&job.SlotTime,
		&job.PlanID,
		&job.TargetStock,
		&job.Status,
		&job.Attempts,
		&job.ConsecutiveTimeouts,
		&job.CurrentStock,
		&job.Uploaded,
		&job.LastError,
		&job.FailureReason,
		&job.RunToken,
		&job.LockedUntil,
		&job.UpdatedAt,
		&job.FinishedAt,
	)
	return job, err
}

func runClaimedSaleCardRestockJob(job saleCardRestockJob, service *tokens.Service) {
	templates := saleCardPlanTemplates()
	plan, ok := templates[job.PlanID]
	if !ok {
		failSaleCardRestockJob(job.ID, "未知上架计划: "+job.PlanID, false)
		return
	}
	plan.TargetStock = job.TargetStock
	plan.Count = 0
	ctx, cancel := context.WithTimeout(context.Background(), saleCardRestockTimeout)
	result, err := generateAndUploadSaleCards(ctx, service, plan)
	cancel()
	if err == nil {
		finalCurrent := result.CurrentStock
		if result.Uploaded > 0 {
			if verified, verifyErr := verifySaleCardRestockStock(plan); verifyErr == nil {
				finalCurrent = verified
			} else {
				failSaleCardRestockJob(job.ID, "补卡后库存确认失败: "+saleCardShopErrorMessage(verifyErr), false)
				return
			}
		}
		if finalCurrent >= job.TargetStock {
			succeedSaleCardRestockJob(job.ID, finalCurrent, result.Uploaded)
			return
		}
		requeueSaleCardRestockJob(job.ID, finalCurrent, result.Uploaded, fmt.Sprintf("库存仍未补齐：当前 %d / 目标 %d", finalCurrent, job.TargetStock), false, job.ConsecutiveTimeouts)
		return
	}
	closeSaleCardRestockHTTPConnections(service)
	if current, verifyErr := verifySaleCardRestockStock(plan); verifyErr == nil && current >= job.TargetStock {
		succeedSaleCardRestockJob(job.ID, current, result.Uploaded)
		return
	}
	if saleCardRestockErrorIsTimeout(err) {
		nextTimeouts := job.ConsecutiveTimeouts + 1
		message := fmt.Sprintf("补卡任务超时：%v", err)
		if nextTimeouts >= saleCardRestockMaxTimeouts {
			failSaleCardRestockJob(job.ID, fmt.Sprintf("连续 %d 次超时，最后错误：%v", nextTimeouts, err), true)
			return
		}
		requeueSaleCardRestockJob(job.ID, result.CurrentStock, result.Uploaded, message, true, nextTimeouts)
		return
	}
	failSaleCardRestockJob(job.ID, saleCardGenerationFailureMessage(err), false)
}

func verifySaleCardRestockStock(plan SaleCardPlan) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), saleCardRestockVerifyTimeout)
	defer cancel()
	current, err := queryMCYUsableStock(ctx, plan.ItemID, plan.SKUID)
	if err != nil {
		closeSaleCardRestockHTTPConnections(nil)
		return 0, err
	}
	return current, nil
}

func succeedSaleCardRestockJob(id int64, currentStock, uploaded int) {
	if db == nil {
		return
	}
	if _, err := db.Exec(`UPDATE sale_card_restock_jobs
		SET status=?, current_stock=?, uploaded=uploaded+?, last_error=NULL, failure_reason=NULL,
			run_token=NULL, locked_until=0, finished_at=datetime('now'), updated_at=datetime('now')
		WHERE id=?`,
		saleCardRestockStatusSucceeded, currentStock, uploaded, id,
	); err != nil {
		fmt.Printf("[sale-card] mark restock job succeeded failed: %v\n", err)
	}
}

func requeueSaleCardRestockJob(id int64, currentStock, uploaded int, message string, timeout bool, consecutiveTimeouts int) {
	if db == nil {
		return
	}
	status := saleCardRestockStatusPending
	if timeout && consecutiveTimeouts >= saleCardRestockMaxTimeouts {
		status = saleCardRestockStatusFailed
	}
	retryAt := time.Now().Add(saleCardRestockRetryDelay).Unix()
	if _, err := db.Exec(`UPDATE sale_card_restock_jobs
		SET status=?, consecutive_timeouts=?, current_stock=?, uploaded=uploaded+?, last_error=?,
			failure_reason=CASE WHEN ? THEN ? ELSE failure_reason END,
			run_token=NULL, locked_until=?, updated_at=datetime('now'),
			finished_at=CASE WHEN ? THEN datetime('now') ELSE finished_at END
		WHERE id=?`,
		status, consecutiveTimeouts, currentStock, uploaded, message,
		status == saleCardRestockStatusFailed, message,
		retryAt, status == saleCardRestockStatusFailed, id,
	); err != nil {
		fmt.Printf("[sale-card] requeue restock job failed: %v\n", err)
	}
}

func failSaleCardRestockJob(id int64, reason string, timeout bool) {
	if db == nil {
		return
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "补卡失败"
	}
	incrementTimeout := 0
	if timeout {
		incrementTimeout = 1
	}
	if _, err := db.Exec(`UPDATE sale_card_restock_jobs
		SET status=?, consecutive_timeouts=consecutive_timeouts+?, last_error=?, failure_reason=?,
			run_token=NULL, locked_until=0, finished_at=datetime('now'), updated_at=datetime('now')
		WHERE id=?`,
		saleCardRestockStatusFailed, incrementTimeout, reason, reason, id,
	); err != nil {
		fmt.Printf("[sale-card] mark restock job failed failed: %v\n", err)
	}
}

func saleCardRestockErrorIsTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "client.timeout") ||
		strings.Contains(msg, "timeout")
}

func closeSaleCardRestockHTTPConnections(service *tokens.Service) {
	if mcyHTTPClient != nil {
		mcyHTTPClient.CloseIdleConnections()
	}
	if service != nil && service.Client != nil && service.Client.HTTPClient != nil {
		service.Client.HTTPClient.CloseIdleConnections()
	}
}

func loadSaleCardRestockStatus() SaleCardRestockStatus {
	if db == nil {
		return SaleCardRestockStatus{Jobs: []SaleCardRestockJobStatus{}}
	}
	rows, err := db.Query(`SELECT id, biz_date, slot_group, slot_time, plan_id, target_stock, status,
			attempts, consecutive_timeouts, current_stock, uploaded, last_error, failure_reason,
			run_token, locked_until, updated_at, finished_at
		FROM sale_card_restock_jobs
		ORDER BY id DESC
		LIMIT ?`, saleCardRestockStatusReportSize)
	if err != nil {
		fmt.Printf("[sale-card] load restock status failed: %v\n", err)
		return SaleCardRestockStatus{Jobs: []SaleCardRestockJobStatus{}}
	}
	defer rows.Close()
	out := []SaleCardRestockJobStatus{}
	for rows.Next() {
		job, err := scanSaleCardRestockJob(rows)
		if err != nil {
			fmt.Printf("[sale-card] scan restock status failed: %v\n", err)
			continue
		}
		out = append(out, SaleCardRestockJobStatus{
			ID:                  job.ID,
			BizDate:             job.BizDate,
			SlotGroup:           job.SlotGroup,
			SlotTime:            job.SlotTime,
			PlanID:              job.PlanID,
			TargetStock:         job.TargetStock,
			Status:              job.Status,
			Attempts:            job.Attempts,
			ConsecutiveTimeouts: job.ConsecutiveTimeouts,
			CurrentStock:        job.CurrentStock,
			Uploaded:            job.Uploaded,
			LastError:           restockNullString(job.LastError),
			FailureReason:       restockNullString(job.FailureReason),
			UpdatedAt:           restockNullString(job.UpdatedAt),
			FinishedAt:          restockNullString(job.FinishedAt),
		})
	}
	return SaleCardRestockStatus{Jobs: out}
}

func restockNullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}
