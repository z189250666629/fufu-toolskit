package combine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

func (a *App) executeMerge(ctx context.Context, p ExecuteMergeParams) (MergeResult, error) {
	validate := func(tokens []ResolvedToken) error {
		return validateExecuteMergeRequest(p, tokens)
	}
	var onProgress func(MergeJobPatch)
	if p.JobID != "" {
		onProgress = func(patch MergeJobPatch) { a.setMergeJob(p.JobID, patch) }
	}
	var quota *int64
	if p.Role == RoleAdmin && p.CustomQuota {
		quota = p.TotalQuota
	}
	name := ""
	if p.Role != RoleGuest {
		name = strings.TrimSpace(p.Name)
	}
	return a.mergeCards(ctx, MergeCardParams{Keys: p.Keys, IntervalUnit: p.IntervalUnit, Quota: quota, Name: name, Role: p.Role, JobID: p.JobID, Validate: validate, OnProgress: onProgress})
}

func (a *App) mergeCards(ctx context.Context, p MergeCardParams) (result MergeResult, err error) {
	update := p.OnProgress
	if update == nil {
		update = func(MergeJobPatch) {}
	}

	mergeID, err := a.createMergeTrace(ctx, p.JobID, p.Role, p.IntervalUnit)
	if err != nil {
		return MergeResult{}, err
	}
	if mergeID != 0 {
		update(MergeJobPatch{MergeID: &mergeID})
	}
	createdID := 0
	rollbackAttempted := false
	rollbackSucceeded := false
	rollbackNote := ""
	mergeCompleted := false
	deletionStarted := false
	oldDeleted := 0
	attemptRollback := func(reason string) rollbackState {
		if createdID == 0 || rollbackAttempted {
			return rollbackState{rollbackAttempted, rollbackSucceeded, rollbackNote}
		}
		rollbackAttempted = true
		update(MergeJobPatch{Status: strp("rollback"), StepText: strp("回滚新卡中..."), Total: intp(1), Current: intp(0)})
		a.setTraceStatus(context.Background(), mergeID, "rollback")
		ok, res, delErr := a.deleteToken(context.Background(), createdID)
		rollbackSucceeded = ok && delErr == nil
		if rollbackSucceeded {
			rollbackNote = fmt.Sprintf("已回滚新卡（%s）", reason)
		} else {
			msg := "未知错误"
			if delErr != nil {
				msg = delErr.Error()
			} else {
				msg = upstreamStatusMessage(res, "删除失败")
			}
			rollbackNote = fmt.Sprintf("新卡回滚失败（%s）：%s", reason, msg)
		}
		a.setTraceRollback(context.Background(), mergeID, rollbackSucceeded, rollbackNote)
		update(MergeJobPatch{Current: intp(1)})
		return rollbackState{rollbackAttempted, rollbackSucceeded, rollbackNote}
	}
	defer func() {
		if err != nil {
			if createdID != 0 && !mergeCompleted && !rollbackAttempted && (!deletionStarted || oldDeleted == 0) {
				attemptRollback("合并异常")
			}
			if rollbackAttempted && !rollbackSucceeded && rollbackNote != "" && !strings.Contains(err.Error(), rollbackNote) {
				err = fmt.Errorf("%s %s", strings.TrimSpace(err.Error()), rollbackNote)
			}
			a.finishTrace(context.Background(), mergeID, "error", err.Error())
			return
		}
		if mergeCompleted {
			a.finishTrace(context.Background(), mergeID, "done", "")
		}
	}()

	a.setTraceStatus(ctx, mergeID, "resolving")
	sourceTokens, err := a.resolveTokensStrict(ctx, p.Keys)
	if err != nil {
		return MergeResult{}, err
	}
	ids := uniqueIDs(sourceTokens)
	if len(ids) != len(sourceTokens) {
		return MergeResult{}, errors.New("存在重复的 key，请勿提交相同的卡密")
	}
	if !a.acquireMergeLock(ids) {
		return MergeResult{}, errors.New("这些卡正在合并中，请稍后再试")
	}
	defer a.releaseMergeLock(ids)
	verifiedQuota := int64(0)
	verified := []ResolvedToken{}
	update(MergeJobPatch{Status: strp("verifying"), StepText: strp("校验额度中..."), Total: intp(len(ids)), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "verifying")
	for i, id := range ids {
		t, e := a.fetchVerifiedToken(ctx, id)
		if e != nil {
			return MergeResult{}, e
		}
		req := findResolvedByID(sourceTokens, id)
		if req == nil {
			return MergeResult{}, fmt.Errorf("Token %d 校验失败", id)
		}
		if strings.TrimPrefix(t.Key, "sk-") != strings.TrimPrefix(req.Key, "sk-") {
			return MergeResult{}, fmt.Errorf("%s 校验失败，请重试", displayKey(req.Key))
		}
		if t.Status != 1 {
			return MergeResult{}, fmt.Errorf("%s 已被禁用，无法参与合卡", displayKey(t.Key))
		}
		verified = append(verified, t)
		verifiedQuota += t.RemainQuota
		if e := a.upsertTraceToken(ctx, mergeID, "source", t); e != nil {
			return MergeResult{}, e
		}
		update(MergeJobPatch{Current: intp(i + 1)})
	}
	if p.Validate != nil {
		if e := p.Validate(verified); e != nil {
			return MergeResult{}, e
		}
	}
	finalQuota := verifiedQuota
	if p.Quota != nil {
		finalQuota = *p.Quota
	}
	if finalQuota <= 0 {
		return MergeResult{}, errors.New("合并额度无效")
	}
	finalName := strings.TrimSpace(p.Name)
	if finalName == "" {
		finalName = strconv.FormatInt(int64(math.Round(float64(finalQuota)/float64(a.quotaUnit))), 10)
	}
	finalGroup := majorityGroup(verified)
	uniqueName := fmt.Sprintf("merge-%d-%s", time.Now().UnixMilli(), randomBase36(6))
	a.setTraceFinal(ctx, mergeID, finalQuota, finalName, finalGroup)

	update(MergeJobPatch{Status: strp("creating"), StepText: strp("创建新卡中..."), Total: intp(1), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "creating")
	body := map[string]any{"name": uniqueName, "remain_quota": finalQuota, "unlimited_quota": false, "expired_time": -1, "group": finalGroup, "interval_quota": finalQuota, "interval_time": -1, "trigger_last_time": 0, "interval_unit": p.IntervalUnit}
	res, _, e := a.createToken(ctx, body)
	if e != nil {
		return MergeResult{}, e
	}
	if !res.OK() {
		return MergeResult{}, errors.New(upstreamStatusMessage(res, "新卡创建失败"))
	}
	update(MergeJobPatch{Current: intp(1)})

	update(MergeJobPatch{Status: strp("renaming"), StepText: strp("整理新卡信息中..."), Total: intp(1), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "renaming")
	token, e := a.searchTokenByName(ctx, uniqueName)
	if e != nil {
		return MergeResult{}, e
	}
	if token == nil || token.ID == 0 {
		return MergeResult{}, errors.New("新卡创建成功但未找到，请稍后人工检查")
	}
	newCard := cloneMap(token.Raw)
	createdID = toInt(newCard["id"])
	a.setTraceCreatedCard(ctx, mergeID, createdID)
	newCard["name"] = finalName
	res, _, e = a.updateTokenRaw(ctx, newCard)
	if e != nil || !res.OK() {
		rb := attemptRollback("重命名失败")
		if rb.succeeded {
			return MergeResult{}, errors.New("新卡重命名失败，已回滚")
		}
		return MergeResult{}, fmt.Errorf("新卡重命名失败，且回滚失败：%s", rb.note)
	}
	resultTraceToken := tokenFromRaw(newCard)
	if e := a.upsertTraceToken(ctx, mergeID, "result", resultTraceToken); e != nil {
		log.Printf("trace result token insert failed: %v", e)
	}
	update(MergeJobPatch{Current: intp(1)})

	update(MergeJobPatch{Status: strp("deleting"), StepText: strp("删卡中..."), Total: intp(len(verified)), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "deleting")
	a.setTraceDeleteStarted(ctx, mergeID)
	deletionStarted = true
	deleteResults := []DeleteResult{}
	deleteFailures := []string{}
	for i, t := range verified {
		ok, res, delErr := a.deleteToken(ctx, t.ID)
		deleteResults = append(deleteResults, DeleteResult{ID: t.ID, Key: t.Key, OK: ok && delErr == nil})
		deleteMessage := ""
		if delErr != nil || !ok {
			if delErr != nil {
				deleteMessage = delErr.Error()
			} else {
				deleteMessage = upstreamStatusMessage(res, "删除失败")
			}
			deleteFailures = append(deleteFailures, displayKey(t.Key))
		} else {
			oldDeleted++
			a.setTraceDeletedCount(ctx, mergeID, oldDeleted)
		}
		a.setTraceTokenDeleteResult(ctx, mergeID, t, ok && delErr == nil, deleteMessage)
		update(MergeJobPatch{Current: intp(i + 1)})
	}
	if len(deleteFailures) > 0 {
		failed := strings.Join(deleteFailures, "、")
		if oldDeleted == 0 {
			rb := attemptRollback("旧卡删除失败")
			if rb.succeeded {
				return MergeResult{}, fmt.Errorf("旧卡删除失败：%s。未删除任何旧卡，已回滚新卡。", failed)
			}
			return MergeResult{}, fmt.Errorf("旧卡删除失败：%s。新卡回滚失败，请立即人工检查。%s", failed, rb.note)
		}
		return MergeResult{}, fmt.Errorf("旧卡删除不完整：%s。已保留新卡以避免额度丢失，请立即人工清理剩余旧卡。", failed)
	}

	result = MergeResult{Success: true, NewCard: NewCardResult{Key: ensureFullKey(getString(newCard, "key")), Name: getString(newCard, "name"), RemainQuota: int64OrDefault(toInt64(newCard["remain_quota"]), finalQuota), IntervalUnit: intOrDefault(toInt(newCard["interval_unit"]), p.IntervalUnit), Group: stringOrDefault(getString(newCard, "group"), finalGroup)}, DeleteResults: deleteResults}
	mergeCompleted = true
	update(MergeJobPatch{Status: strp("done"), StepText: strp("合并完成"), Result: result, HasResult: true, Total: intp(len(verified)), Current: intp(len(verified))})
	return result, nil
}

func (a *App) resolveTokensForSearch(ctx context.Context, raw []string) ([]string, []ResolvedToken, []string, error) {
	keys := normalizeKeys(raw)
	results, err := a.searchTokensConcurrent(ctx, keys)
	if err != nil {
		return keys, nil, nil, err
	}
	found := []ResolvedToken{}
	missing := []string{}
	for _, r := range results {
		if r.Found != nil {
			found = append(found, *r.Found)
		} else {
			missing = append(missing, r.Key)
		}
	}
	return keys, found, missing, nil
}

func (a *App) resolveTokensStrict(ctx context.Context, raw []string) ([]ResolvedToken, error) {
	keys := normalizeKeys(raw)
	if len(keys) == 0 {
		return nil, errors.New("No keys provided")
	}
	results, err := a.searchTokensConcurrent(ctx, keys)
	if err != nil {
		return nil, err
	}
	missing := []string{}
	found := make([]ResolvedToken, 0, len(keys))
	for _, r := range results {
		if r.Found == nil {
			missing = append(missing, r.Key)
		} else {
			found = append(found, *r.Found)
		}
	}
	if len(missing) > 0 {
		shown := []string{}
		for _, k := range missing {
			shown = append(shown, displayKey(k))
		}
		return nil, fmt.Errorf("未找到令牌: %s", strings.Join(shown, ", "))
	}
	return found, nil
}
