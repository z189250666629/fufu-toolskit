package activityapp

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	rewardPlanPoolSize         = 3
	rewardPlanLeaseStateIdle   = "idle"
	rewardPlanLeaseStateLeased = "leased"
)

type rewardPlanLease struct {
	Slot                    int
	PlanID                  int64
	State                   string
	CardKey                 string
	UserID                  int64
	SourceSubscriptionID    int64
	RewardQuota             int64
	DurationUnit            string
	DurationValue           int64
	CustomSeconds           int64
	BaselineSubscriptionIDs []int64
	NextBindAt              int64
	CreatedAt               int64
	UpdatedAt               int64
}

func seedRewardPlanPool(d *sql.DB) error {
	if d == nil {
		return nil
	}
	now := time.Now().Unix()
	for slot := 1; slot <= rewardPlanPoolSize; slot++ {
		if _, err := d.Exec(
			`INSERT OR IGNORE INTO reward_plan_pool (slot,state,baseline_subscription_ids,created_at,updated_at) VALUES (?,?,?,?,?)`,
			slot,
			rewardPlanLeaseStateIdle,
			"[]",
			now,
			now,
		); err != nil {
			return err
		}
	}
	return nil
}

func acquireOrResumeRewardPlanLease(cardKey string) (rewardPlanLease, error) {
	cardKey = strings.TrimSpace(cardKey)
	if db == nil {
		return rewardPlanLease{}, fmt.Errorf("reward plan pool database is not configured")
	}
	if cardKey == "" {
		return rewardPlanLease{}, fmt.Errorf("reward plan pool card key is missing")
	}
	tx, err := db.Begin()
	if err != nil {
		return rewardPlanLease{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if lease, ok, err := lookupRewardPlanLeaseTx(tx, `lease_card_key=?`, cardKey); err != nil {
		return rewardPlanLease{}, err
	} else if ok {
		if err := tx.Commit(); err != nil {
			return rewardPlanLease{}, err
		}
		committed = true
		return lease, nil
	}
	lease, ok, err := lookupRewardPlanLeaseTx(tx, `state=?`, rewardPlanLeaseStateIdle)
	if err != nil {
		return rewardPlanLease{}, err
	}
	if !ok {
		return rewardPlanLease{}, fmt.Errorf("reward plan pool is busy, please retry")
	}
	now := time.Now().Unix()
	res, err := tx.Exec(
		`UPDATE reward_plan_pool
		 SET state=?, lease_card_key=?, lease_user_id=0, source_subscription_id=0, reward_quota=0,
		     duration_unit='', duration_value=0, custom_seconds=0, baseline_subscription_ids='[]',
		     next_bind_at=0, updated_at=?
		 WHERE slot=? AND state=?`,
		rewardPlanLeaseStateLeased,
		cardKey,
		now,
		lease.Slot,
		rewardPlanLeaseStateIdle,
	)
	if err != nil {
		return rewardPlanLease{}, err
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return rewardPlanLease{}, fmt.Errorf("reward plan pool lease conflict, please retry")
	}
	if updated, ok, err := lookupRewardPlanLeaseTx(tx, `slot=?`, lease.Slot); err != nil {
		return rewardPlanLease{}, err
	} else if ok {
		lease = updated
	}
	if err := tx.Commit(); err != nil {
		return rewardPlanLease{}, err
	}
	committed = true
	return lease, nil
}

func saveRewardPlanLeasePlanID(cardKey string, planID int64) error {
	if db == nil || strings.TrimSpace(cardKey) == "" || planID <= 0 {
		return nil
	}
	_, err := db.Exec(
		`UPDATE reward_plan_pool SET plan_id=?, updated_at=? WHERE lease_card_key=?`,
		planID,
		time.Now().Unix(),
		cardKey,
	)
	return err
}

func prepareRewardPlanLeaseForBind(card Card, lease *rewardPlanLease, planID, rewardQuota int64, duration rewardPlanDuration, baselineIDs []int64) error {
	if db == nil {
		return fmt.Errorf("reward plan pool database is not configured")
	}
	if lease == nil || lease.Slot <= 0 {
		return fmt.Errorf("reward plan pool lease is missing")
	}
	duration = normalizeRewardPlanDuration(duration)
	baselineJSON, err := marshalRewardPlanBaselineIDs(baselineIDs)
	if err != nil {
		return err
	}
	nextBindAt := time.Now().Add(subscriptionRewardBindRetryCooldown).Unix()
	now := time.Now().Unix()
	_, err = db.Exec(
		`UPDATE reward_plan_pool
		 SET plan_id=?, lease_user_id=?, source_subscription_id=?, reward_quota=?, duration_unit=?, duration_value=?, custom_seconds=?,
		     baseline_subscription_ids=?, next_bind_at=?, updated_at=?
		 WHERE slot=? AND lease_card_key=?`,
		planID,
		card.UserID.Int64,
		card.SubscriptionID.Int64,
		rewardQuota,
		duration.Unit,
		duration.Value,
		duration.CustomSeconds,
		baselineJSON,
		nextBindAt,
		now,
		lease.Slot,
		card.CardKey,
	)
	if err != nil {
		return err
	}
	lease.PlanID = planID
	lease.UserID = card.UserID.Int64
	lease.SourceSubscriptionID = card.SubscriptionID.Int64
	lease.RewardQuota = rewardQuota
	lease.DurationUnit = duration.Unit
	lease.DurationValue = duration.Value
	lease.CustomSeconds = duration.CustomSeconds
	lease.BaselineSubscriptionIDs = append([]int64(nil), baselineIDs...)
	lease.NextBindAt = nextBindAt
	lease.UpdatedAt = now
	return nil
}

func releaseRewardPlanLease(cardKey string) error {
	if db == nil || strings.TrimSpace(cardKey) == "" {
		return nil
	}
	_, err := db.Exec(
		`UPDATE reward_plan_pool
		 SET state=?, lease_card_key=NULL, lease_user_id=0, source_subscription_id=0, reward_quota=0,
		     duration_unit='', duration_value=0, custom_seconds=0, baseline_subscription_ids='[]',
		     next_bind_at=0, updated_at=?
		 WHERE lease_card_key=?`,
		rewardPlanLeaseStateIdle,
		time.Now().Unix(),
		cardKey,
	)
	return err
}

func lookupRewardPlanLeaseTx(tx *sql.Tx, where string, arg any) (rewardPlanLease, bool, error) {
	query := `SELECT slot,plan_id,state,lease_card_key,lease_user_id,source_subscription_id,reward_quota,duration_unit,duration_value,custom_seconds,baseline_subscription_ids,next_bind_at,created_at,updated_at FROM reward_plan_pool WHERE ` + where + ` ORDER BY slot ASC LIMIT 1`
	var lease rewardPlanLease
	var cardKey sql.NullString
	var baselineRaw string
	err := tx.QueryRow(query, arg).Scan(
		&lease.Slot,
		&lease.PlanID,
		&lease.State,
		&cardKey,
		&lease.UserID,
		&lease.SourceSubscriptionID,
		&lease.RewardQuota,
		&lease.DurationUnit,
		&lease.DurationValue,
		&lease.CustomSeconds,
		&baselineRaw,
		&lease.NextBindAt,
		&lease.CreatedAt,
		&lease.UpdatedAt,
	)
	if err == nil {
		if cardKey.Valid {
			lease.CardKey = cardKey.String
		}
		lease.BaselineSubscriptionIDs, err = unmarshalRewardPlanBaselineIDs(baselineRaw)
		if err != nil {
			return rewardPlanLease{}, false, err
		}
		return lease, true, nil
	}
	if err == sql.ErrNoRows {
		return rewardPlanLease{}, false, nil
	}
	return rewardPlanLease{}, false, err
}

func marshalRewardPlanBaselineIDs(ids []int64) (string, error) {
	if len(ids) == 0 {
		return "[]", nil
	}
	buf, err := json.Marshal(ids)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func unmarshalRewardPlanBaselineIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []int64{}, nil
	}
	var ids []int64
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}
