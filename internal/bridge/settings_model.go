package bridge

import "time"

// Settings is the single persisted row holding Core's live Bridge
// connection state, so an administrator can connect/rotate/disconnect
// Bridge from Core's own settings UI without editing config files or
// restarting the process. Falls back to config.BridgeConfig (env/YAML) on
// first boot -- see Service.bootstrap.
//
// InstanceToken is a secret. It is stored here (not logged, never returned
// by any Service/handler method after it is first set) because Core must
// keep re-sending it as the Authorization header on every Bridge request;
// this mirrors plan §15's "If configuration is written through settings,
// encrypt it at rest where practical" -- encryption-at-rest for this
// column is a known gap, flagged in the work log, not implemented this
// session (see donetick-bridge/work-log.md Phase 5 notes).
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
