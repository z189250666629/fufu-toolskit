package activityapp

import (
	"database/sql"
	"errors"
	"strings"
)

type rewardIssuanceRecord struct {
	CardKey              string
	UserID               int64
	SourceSubscriptionID int64
	RewardPlanID         int64
	RewardSubscriptionID int64
	RewardQuota          int64
}

func lookupRewardIssuance(cardKey string) (rewardIssuanceRecord, bool, error) {
	if db == nil || strings.TrimSpace(cardKey) == "" {
		return rewardIssuanceRecord{}, false, nil
	}
	var rec rewardIssuanceRecord
	err := db.QueryRow(
		`SELECT card_key,user_id,source_subscription_id,reward_plan_id,reward_subscription_id,reward_quota FROM reward_issuance WHERE card_key=?`,
		cardKey,
	).Scan(
		&rec.CardKey,
		&rec.UserID,
		&rec.SourceSubscriptionID,
		&rec.RewardPlanID,
		&rec.RewardSubscriptionID,
		&rec.RewardQuota,
	)
	if err == nil {
		return rec, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return rewardIssuanceRecord{}, false, nil
	}
	return rewardIssuanceRecord{}, false, err
}

func saveRewardIssuance(card Card, rewardPlanID, rewardSubscriptionID, rewardQuota int64) error {
	if db == nil {
		return nil
	}
	if strings.TrimSpace(card.CardKey) == "" || rewardPlanID <= 0 || rewardSubscriptionID <= 0 {
		return nil
	}
	_, err := db.Exec(
		`INSERT INTO reward_issuance (card_key,user_id,source_subscription_id,reward_plan_id,reward_subscription_id,reward_quota,updated_at)
		 VALUES (?,?,?,?,?,?,datetime('now'))
		 ON CONFLICT(card_key) DO UPDATE SET
		   user_id=excluded.user_id,
		   source_subscription_id=excluded.source_subscription_id,
		   reward_plan_id=excluded.reward_plan_id,
		   reward_subscription_id=excluded.reward_subscription_id,
		   reward_quota=excluded.reward_quota,
		   updated_at=datetime('now')`,
		card.CardKey,
		card.UserID.Int64,
		card.SubscriptionID.Int64,
		rewardPlanID,
		rewardSubscriptionID,
		rewardQuota,
	)
	return err
}
