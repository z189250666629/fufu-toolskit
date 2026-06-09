package combine

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

func (a *App) handleAuth(w http.ResponseWriter, r *http.Request) {
	var p struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	matched, ok := matchRoleByPassword(a.passwords, p.Password)
	if !ok {
		time.Sleep(time.Second)
		writeJSON(w, 401, map[string]string{"error": "密码错误"})
		return
	}
	token, err := randomHex(24)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "生成会话失败"})
		return
	}
	now := time.Now()
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
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if len(p.Keys) == 0 {
		writeJSON(w, 400, map[string]string{"error": "No keys provided"})
		return
	}
	keys, found, missing, err := a.resolveTokensForSearch(r.Context(), p.Keys)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	traceResults, err := a.traceResultsForKeys(r.Context(), p.Keys)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	elig := evaluatePublicMergeEligibility(found)
	writeJSON(w, 200, map[string]any{
		"found": found, "missing": missing, "quotaUnit": a.quotaUnit, "searched": len(keys),
		"concurrency": min(searchConcurrency, len(keys)), "elapsedMs": time.Since(started).Milliseconds(),
		"publicMergeEligibility": map[string]any{"eligible": elig.Eligible, "reasons": elig.Reasons, "targetUnit": publicTargetUnit},
		"traceResults":           traceResults,
	})
}

func (a *App) handleMerge(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	var p MergePayload
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	role := roleFromContext(r.Context())
	jobID, err := randomHex(16)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "创建任务失败"})
		return
	}
	a.setMergeJob(jobID, buildQueuedMergeJobPatch(len(p.Keys), role, "准备合并..."))
	go a.runMergeJob(jobID, p, role)
	writeJSON(w, 200, buildMergeAcceptedResponse(jobID))
}

func (a *App) handlePublicMerge(w http.ResponseWriter, r *http.Request) {
	a.cleanMergeJobs()
	var p struct {
		Keys []string `json:"keys"`
	}
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	jobID, err := randomHex(16)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "创建任务失败"})
		return
	}
	role := RoleGuest
	a.setMergeJob(jobID, buildQueuedMergeJobPatch(len(p.Keys), role, "准备普通合卡..."))
	go a.runMergeJob(jobID, MergePayload{Keys: p.Keys, IntervalUnit: publicTargetUnit}, RoleGuest)
	writeJSON(w, 200, buildMergeAcceptedResponse(jobID))
}

func (a *App) runMergeJob(jobID string, p MergePayload, role Role) {
	defer func() {
		if x := recover(); x != nil {
			a.setMergeJob(jobID, MergeJobPatch{Status: strp("error"), StepText: strp("合并失败"), Error: strp(fmt.Sprint(x))})
		}
	}()
	_, err := a.executeMerge(context.Background(), ExecuteMergeParams{Keys: p.Keys, IntervalUnit: p.IntervalUnit, TotalQuota: p.TotalQuota, Name: p.Name, CustomQuota: p.CustomQuota, Role: role, JobID: jobID})
	if err != nil {
		a.setMergeJob(jobID, MergeJobPatch{Status: strp("error"), StepText: strp("合并失败"), Error: strp(err.Error())})
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
	writeJSON(w, 200, job)
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
	if err := decodeJSON(r.Body, &p); err != nil {
		writeJSON(w, 400, map[string]string{"error": "请求格式错误"})
		return
	}
	if p.Count < 1 || p.Count > 100 || p.Quota <= 0 || p.IntervalUnit == 0 {
		writeJSON(w, 400, map[string]string{"error": "参数无效"})
		return
	}
	totalQuota := int64(math.Round(p.Quota * float64(a.quotaUnit)))
	group := normalizeGenerateGroup(p.Group)
	keys := []string{}
	errs := []string{}
	for i := 0; i < p.Count; i++ {
		uniqueName := fmt.Sprintf("gen-%d-%s", time.Now().UnixMilli(), randomBase36(6))
		res, _, err := a.createToken(r.Context(), buildGeneratedTokenCreateBody(uniqueName, totalQuota, group, p.IntervalUnit))
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %s", i+1, err))
			continue
		}
		if !res.OK() {
			errs = append(errs, fmt.Sprintf("#%d: 创建失败", i+1))
			continue
		}
		token, err := a.searchTokenByName(r.Context(), uniqueName)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %s", i+1, err))
			continue
		}
		if token == nil {
			errs = append(errs, fmt.Sprintf("#%d: 创建成功但未找到", i+1))
			continue
		}
		card := cloneMap(token.Raw)
		card["name"] = strconv.FormatFloat(p.Quota, 'f', -1, 64)
		if res, _, err := a.updateTokenRaw(r.Context(), card); err != nil {
			errs = append(errs, fmt.Sprintf("#%d: %s", i+1, err))
			continue
		} else if !res.OK() {
			errs = append(errs, fmt.Sprintf("#%d: 重命名失败", i+1))
			continue
		}
		tokenID := toInt(card["id"])
		verifiedToken, err := a.fetchVerifiedToken(r.Context(), tokenID)
		if err != nil {
			errs = append(errs, fmt.Sprintf("#%d: 创建成功但复查失败: %s", i+1, err))
			continue
		}
		if err := a.upsertGeneratedToken(r.Context(), verifiedToken); err != nil {
			log.Printf("generated token cache insert failed: %v", err)
		}
		keys = append(keys, verifiedToken.Key)
	}
	writeJSON(w, 200, map[string]any{"keys": keys, "errors": errs})
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
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, statusOrDefault(res.StatusCode, 500), map[string]string{"error": upstreamStatusMessage(res, "删除失败")})
		return
	}
	a.deleteGeneratedTokenCacheByID(r.Context(), id)
	writeJSON(w, 200, map[string]bool{"success": true})
}
