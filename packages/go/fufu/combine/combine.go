package combine

import (
	"time"

	"fufu/auth"
	"fufu/newapi"
)

const (
	searchConcurrency = 6
	sessionTTL        = 4 * time.Hour
	mergeJobTTL       = 30 * time.Minute
	publicSourceUnit  = 3
	publicTargetUnit  = 8
	maxTraceRecords   = 50
)

type Role = auth.Role

const (
	RoleAdmin = auth.RoleAdmin
	RoleUser  = auth.RoleUser
	RoleGuest = auth.RoleGuest
)

type APIResponse = newapi.Response
