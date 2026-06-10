package combine

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

func (a *App) handleAuth(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Password string `json:"password"`
	}
	if err := decodeJSONRequest(w, r, &p); err != nil {
		writeJSONDecodeError(w, err)
		return
	}
	now := time.Now()
	if until, blocked := a.authBlockedUntil(r, now); blocked {
		w.Header().Set("Retry-After", retryAfterSeconds(until, now))
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "登录尝试过多，请稍后重试"})
		return
	}
	matched, ok := matchRoleByPassword(a.passwords, p.Password)
	if !ok {
		a.recordAuthFailure(r, now)
		if a.authFailureDelay > 0 {
			time.Sleep(a.authFailureDelay)
		}
		writeJSON(w, 401, map[string]string{"error": "密码错误"})
		return
	}
	a.clearAuthFailures(r)
	token, err := randomHex(24)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "生成会话失败"})
		return
	}
	now = time.Now()
	a.mu.Lock()
	a.cleanSessionsLocked(now)
	a.sessions[token] = SessionInfo{Expiry: now.Add(sessionTTL), Role: matched}
	a.mu.Unlock()
	writeJSON(w, 200, map[string]any{"token": token, "role": matched})
}

func (a *App) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true, "role": roleFromContext(r.Context())})
}

func (a *App) handleSearchKeys(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	var p struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSONRequest(w, r, &p); err != nil {
		writeJSONDecodeError(w, err)
		return
	}
	keys := normalizeKeys(p.Keys)
	if len(keys) == 0 {
		writeJSON(w, 400, map[string]string{"error": "No keys provided"})
		return
	}
	if len(keys) > maxKeysPerRequest {
		writeTooManyKeysRequest(w)
		return
	}
	keys, found, missing, err := a.resolveTokensForSearch(r.Context(), keys)
	if err != nil {
		log.Printf("combine search token lookup failed: %s", redactError(err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败，请稍后重试"})
		return
	}
	traceResults, err := a.traceResultsForKeys(r.Context(), keys)
	if err != nil {
		log.Printf("combine search trace lookup failed: %s", redactError(err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "查询失败，请稍后重试"})
		return
	}
	writeJSON(w, 200, buildSearchKeysResponse(keys, found, missing, a.quotaUnit, time.Since(started).Milliseconds(), traceResults))
}

func (a *App) handleMerge(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	var p MergePayload
	if err := decodeJSONRequest(w, r, &p); err != nil {
		writeJSONDecodeError(w, err)
		return
	}
	p.Keys = normalizeKeys(p.Keys)
	if len(p.Keys) == 0 {
		writeJSON(w, 400, map[string]string{"error": "No keys provided"})
		return
	}
	if len(p.Keys) > maxKeysPerRequest {
		writeTooManyKeysRequest(w)
		return
	}
	role := roleFromContext(r.Context())
	jobID, err := randomHex(16)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "创建任务失败"})
		return
	}
	if !a.tryQueueMergeJob(jobID, buildQueuedMergeJobPatch(len(p.Keys), role, "准备合并...")) {
		writeMergeBusy(w)
		return
	}
	go a.runMergeJob(jobID, p, role)
	writeJSON(w, 200, buildMergeAcceptedResponse(jobID))
}

func (a *App) handlePublicMerge(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	var p struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSONRequest(w, r, &p); err != nil {
		writeJSONDecodeError(w, err)
		return
	}
	keys := normalizeKeys(p.Keys)
	if len(keys) == 0 {
		writeJSON(w, 400, map[string]string{"error": "No keys provided"})
		return
	}
	if len(keys) > maxKeysPerRequest {
		writeTooManyKeysRequest(w)
		return
	}
	jobID, err := randomHex(16)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "创建任务失败"})
		return
	}
	role := RoleGuest
	if !a.tryQueueMergeJob(jobID, buildQueuedMergeJobPatch(len(keys), role, "准备普通合卡...")) {
		writeMergeBusy(w)
		return
	}
	go a.runMergeJob(jobID, MergePayload{Keys: keys, IntervalUnit: publicTargetUnit}, RoleGuest)
	writeJSON(w, 200, buildMergeAcceptedResponse(jobID))
}

func (a *App) runMergeJob(jobID string, p MergePayload, role Role) {
	defer func() {
		if x := recover(); x != nil {
			a.setMergeJob(jobID, MergeJobPatch{Status: strp("error"), StepText: strp("合并失败"), Error: strp(redactTraceDiagnostic(fmt.Sprint(x)))})
		}
	}()
	_, err := a.executeMerge(context.Background(), buildRunMergeJobParams(jobID, p, role))
	if err != nil {
		a.setMergeJob(jobID, MergeJobPatch{Status: strp("error"), StepText: strp("合并失败"), Error: strp(redactError(err))})
	}
}

func (a *App) handleMergeStatus(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	jobID := mergeStatusJobIDFromPath(r.URL.Path)
	job, ok := a.getMergeJob(jobID)
	if jobID == "" || !ok {
		writeJSON(w, 404, map[string]string{"error": "任务不存在或已过期"})
		return
	}
	if !a.canReadMergeJobStatus(r, job) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未授权"})
		return
	}
	job.Error = safeMergeJobError(job.Error)
	if job.Role == RoleGuest {
		job = publicMergeJobStatus(job)
	}
	writeJSON(w, 200, job)
}

func (a *App) canReadMergeJobStatus(r *http.Request, job MergeJob) bool {
	if job.Role == RoleGuest {
		return true
	}
	if roleFromContext(r.Context()) != "" {
		return true
	}
	_, ok := a.authenticate(r)
	return ok
}

func (a *App) handleGenerate(w http.ResponseWriter, r *http.Request) {
	if roleFromContext(r.Context()) != RoleAdmin {
		writeJSON(w, 403, map[string]string{"error": "无权操作"})
		return
	}
	var p struct {
		Count        int     `json:"count"`
		Quota        float64 `json:"quota"`
		IntervalUnit int     `json:"intervalUnit"`
		Group        string  `json:"group"`
	}
	if err := decodeJSONRequest(w, r, &p); err != nil {
		writeJSONDecodeError(w, err)
		return
	}
	if !validateGenerateParams(p.Count, p.Quota, p.IntervalUnit, a.quotaUnit) {
		writeJSON(w, 400, map[string]string{"error": "参数无效"})
		return
	}
	totalQuota := generateTotalQuota(p.Quota, a.quotaUnit)
	group := normalizeGenerateGroup(p.Group)
	keys := []string{}
	errs := []string{}
	for i := 0; i < p.Count; i++ {
		uniqueName := fmt.Sprintf("gen-%d-%s", time.Now().UnixMilli(), randomBase36(6))
		res, _, err := a.createToken(r.Context(), buildGeneratedTokenCreateBody(uniqueName, totalQuota, group, p.IntervalUnit))
		if err != nil {
			errs = appendGenerateError(errs, i, "生成失败，请稍后重试", err)
			continue
		}
		if !res.OK() {
			errs = append(errs, fmt.Sprintf("#%d: 创建失败", i+1))
			continue
		}
		token, err := a.searchTokenByName(r.Context(), uniqueName)
		if err != nil {
			errs = appendGenerateError(errs, i, "生成成功但查询失败，请稍后重试", err)
			continue
		}
		if token == nil {
			errs = append(errs, fmt.Sprintf("#%d: 创建成功但未找到", i+1))
			continue
		}
		card := cloneMap(token.Raw)
		card["name"] = generateTokenFinalName(p.Quota)
		if res, _, err := a.updateTokenRaw(r.Context(), card); err != nil {
			errs = appendGenerateError(errs, i, "生成成功但重命名失败，请稍后重试", err)
			continue
		} else if !res.OK() {
			errs = append(errs, fmt.Sprintf("#%d: 重命名失败", i+1))
			continue
		}
		tokenID := toInt(card["id"])
		verifiedToken, err := a.fetchVerifiedToken(r.Context(), tokenID)
		if err != nil {
			errs = appendGenerateError(errs, i, "生成成功但复查失败，请稍后重试", err)
			continue
		}
		if err := a.upsertGeneratedToken(r.Context(), verifiedToken); err != nil {
			log.Printf("generated token cache insert failed: %s", redactError(err))
		}
		keys = append(keys, verifiedToken.Key)
	}
	writeJSON(w, 200, map[string]any{"keys": keys, "errors": errs})
}

func appendGenerateError(errs []string, index int, message string, err error) []string {
	log.Printf("combine generate token #%d failed: %s", index+1, redactError(err))
	return append(errs, fmt.Sprintf("#%d: %s", index+1, message))
}

func (a *App) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	role := roleFromContext(r.Context())
	if !canDeleteTokenRole(role) {
		writeJSON(w, 403, map[string]string{"error": "无权删除"})
		return
	}
	id, ok := deleteTokenIDFromPath(r.URL.Path)
	if !ok {
		writeJSON(w, 400, map[string]string{"error": "无效的 token ID"})
		return
	}
	ok, res, err := a.deleteToken(r.Context(), id)
	if err != nil {
		log.Printf("combine delete token failed: %s", redactError(err))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "删除失败，请稍后重试"})
		return
	}
	if !ok {
		writeJSON(w, statusOrDefault(res.StatusCode, 500), map[string]string{"error": upstreamStatusMessage(res, "删除失败")})
		return
	}
	a.deleteGeneratedTokenCacheByID(r.Context(), id)
	writeJSON(w, 200, map[string]bool{"success": true})
}
