package bridge

import "time"

// CoreConfig is the minimal shape Core's config.BridgeConfig must satisfy
// to build a Client/Service. Defined here (instead of importing
// donetick.com/core/config directly) purely to avoid the internal/bridge
// package depending on the top-level config package's own dependency
// graph; callers in main.go still construct this from *config.Config.
type CoreConfig struct {
	Enabled        bool
	BaseURL        string
	InstanceID     string
	InstanceToken  string
	TimeoutSeconds int
}

// NewClientFromCoreConfig builds a Client from Core's bridge.* config
// section. Whether the resulting client is actually usable is
// governed by Client.Enabled(), not by whether this constructor was called
// -- an fx-style always-provide-the-type pattern keeps wiring simple even
// when Bridge is off.
func NewClientFromCoreConfig(cc CoreConfig) *Client {
	timeout := time.Duration(cc.TimeoutSeconds) * time.Second
	return NewClient(Config{
		Enabled:       cc.Enabled,
		BaseURL:       cc.BaseURL,
		InstanceID:    cc.InstanceID,
		InstanceToken: cc.InstanceToken,
		Timeout:       timeout,
	})
}
