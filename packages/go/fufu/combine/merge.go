package combine

import (
	"context"
	"errors"
	"fmt"
	"log"
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
			if shouldAttemptMergeRollbackOnError(createdID, mergeCompleted, rollbackAttempted, deletionStarted, oldDeleted) {
				attemptRollback("合并异常")
			}
			err = appendRollbackNote(err, rollbackState{rollbackAttempted, rollbackSucceeded, rollbackNote})
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
	verified := []ResolvedToken{}
	update(MergeJobPatch{Status: strp("verifying"), StepText: strp("校验额度中..."), Total: intp(len(ids)), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "verifying")
	for i, id := range ids {
		t, e := a.fetchVerifiedToken(ctx, id)
		if e != nil {
			return MergeResult{}, e
		}
		req := findResolvedByID(sourceTokens, id)
		if e := validateVerifiedSourceToken(req, t); e != nil {
			return MergeResult{}, e
		}
		verified = append(verified, t)
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
	target, err := buildMergeTargetPlan(verified, p.Quota, p.Name, a.quotaUnit)
	if err != nil {
		return MergeResult{}, err
	}
	uniqueName := fmt.Sprintf("merge-%d-%s", time.Now().UnixMilli(), randomBase36(6))
	a.setTraceFinal(ctx, mergeID, target.Quota, target.Name, target.Group)

	update(MergeJobPatch{Status: strp("creating"), StepText: strp("创建新卡中..."), Total: intp(1), Current: intp(0)})
	a.setTraceStatus(ctx, mergeID, "creating")
	res, _, e := a.createToken(ctx, buildNewMergeTokenBody(uniqueName, target, p.IntervalUnit))
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
	newCard["name"] = target.Name
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
		outcome := buildMergeDeleteOutcome(t, ok, res, delErr)
		deleteResults = append(deleteResults, outcome.result)
		if outcome.failureKey != "" {
			deleteFailures = append(deleteFailures, outcome.failureKey)
		} else {
			oldDeleted++
			a.setTraceDeletedCount(ctx, mergeID, oldDeleted)
		}
		a.setTraceTokenDeleteResult(ctx, mergeID, t, outcome.result.OK, outcome.traceMessage)
		update(MergeJobPatch{Current: intp(i + 1)})
	}
	if len(deleteFailures) > 0 {
		var rb *rollbackState
		if oldDeleted == 0 {
			state := attemptRollback("旧卡删除失败")
			rb = &state
		}
		return MergeResult{}, formatMergeDeletionFailure(deleteFailures, oldDeleted, rb)
	}

	result = buildMergeResult(newCard, target, p.IntervalUnit, deleteResults)
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
	found, missing := splitSearchTokenResults(results)
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
	found, missing := splitSearchTokenResults(results)
	if err := missingTokenError(missing); err != nil {
		return nil, err
	}
	return found, nil
}
