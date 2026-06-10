package combine

import (
	"time"

	"fufu/auth"
	"fufu/newapi"
)

const (
	searchConcurrency  = 6
	sessionTTL         = 4 * time.Hour
	mergeJobTTL        = 30 * time.Minute
	traceRetention     = 30 * 24 * time.Hour
	maxJSONBodyBytes   = int64(1 << 20)
	maxKeysPerRequest  = 200
	maxActiveMergeJobs = 20
	publicSourceUnit   = 3
	publicTargetUnit   = 8
	maxTraceRecords    = 50
	authFailureLimit   = 5
	authFailureWindow  = 5 * time.Minute
	authFailureLockout = time.Minute
	authFailureDelay   = time.Second
)

type Role = auth.Role

const (
	RoleAdmin = auth.RoleAdmin
	RoleUser  = auth.RoleUser
	RoleGuest = auth.RoleGuest
)

type APIResponse = newapi.Response
