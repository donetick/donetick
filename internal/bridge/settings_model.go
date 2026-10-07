package bridge

import "time"

// Settings is the single persisted row holding Core's live Bridge
// connection state, so an administrator can connect/rotate/disconnect
// Bridge from Core's own settings UI without editing config files or
// restarting the process. Falls back to config.BridgeConfig (env/YAML) on
// first boot -- see Service.bootstrap.

type Settings struct {
	ID             int        `json:"-" gorm:"primaryKey;column:id"`
	Enabled        bool       `json:"enabled" gorm:"column:enabled"`
	BaseURL        string     `json:"baseUrl" gorm:"column:base_url"`
	InstanceID     string     `json:"instanceId" gorm:"column:instance_id"`
	InstanceToken  string     `json:"-" gorm:"column:instance_token"`
	TimeoutSeconds int        `json:"-" gorm:"column:timeout_seconds"`
	ConnectedAt    *time.Time `json:"connectedAt" gorm:"column:connected_at"`
	UpdatedAt      time.Time  `json:"-" gorm:"column:updated_at"`
}

func (Settings) TableName() string { return "bridge_settings" }
