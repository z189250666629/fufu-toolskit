package main

import (
	"fufu/activity"
)

const toolConfigFileName = "tool-config.json"

var unifiedConfig *toolConfigStore

type ToolConfig struct {
	NewAPI     NewAPIAdminConfig `json:"newapi"`
	Navigation NavigationConfig  `json:"navigation"`
	Activity   activity.Config   `json:"activity"`
	MCY        MCYAdminConfig    `json:"mcy"`
}

type NewAPIAdminConfig struct {
	Sites []ManagedAPISiteConfig `json:"sites"`
}

type MCYAdminConfig struct {
	BaseURL        string `json:"baseUrl"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	Cookie         string `json:"cookie,omitempty"`
	LoginEndpoint  string `json:"loginEndpoint,omitempty"`
	UploadEndpoint string `json:"uploadEndpoint,omitempty"`
}

type ManagedSiteURL struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url"`
}

type ManagedAPISiteConfig struct {
	Name                string           `json:"name"`
	Category            string           `json:"category,omitempty"`
	URLs                []ManagedSiteURL `json:"urls"`
	URL                 string           `json:"url,omitempty"`
	Token               string           `json:"token,omitempty"`
	UserID              string           `json:"userId"`
	Kind                string           `json:"kind,omitempty"`
	SkipUserHeader      bool             `json:"skipUserHeader,omitempty"`
	QuotaUnit           int64            `json:"quotaUnit"`
	Currency            string           `json:"currency"`
	RechargeRatio       float64          `json:"rechargeRatio"`
	ChannelListEndpoint string           `json:"channelListEndpoint,omitempty"`
	Note                string           `json:"note,omitempty"`
}

type adminConfigPatch struct {
	NewAPI *struct {
		Sites []ManagedAPISiteConfig `json:"sites"`
	} `json:"newapi"`
	Navigation *NavigationConfig `json:"navigation"`
	Activity   *activity.Config  `json:"activity"`
	MCY        *MCYAdminConfig   `json:"mcy"`
}
