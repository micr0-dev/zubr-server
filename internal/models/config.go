package models

// Channel represents an IRC channel to auto-join
type Channel struct {
	Name  string `json:"name"`
	Key   string `json:"key"`
	Muted bool   `json:"muted"`
}

// Network represents an IRC network configuration
type Network struct {
	UUID                string    `json:"uuid"`
	Name                string    `json:"name"`
	Host                string    `json:"host"`
	Port                int       `json:"port"`
	TLS                 bool      `json:"tls"`
	RejectUnauthorized  bool      `json:"rejectUnauthorized"`
	Password            string    `json:"password"`
	Nick                string    `json:"nick"`
	Username            string    `json:"username"`
	Realname            string    `json:"realname"`
	SASL                string    `json:"sasl"`
	SASLAccount         string    `json:"saslAccount"`
	SASLPassword        string    `json:"saslPassword"`
	Commands            []string  `json:"commands"`
	AwayMessage         string    `json:"awayMessage"`
	LeaveMessage        string    `json:"leaveMessage"`
	Channels            []Channel `json:"channels"`
	ProxyEnabled        bool      `json:"proxyEnabled"`
	ProxyHost           string    `json:"proxyHost"`
	ProxyPort           int       `json:"proxyPort"`
	ProxyUsername       string    `json:"proxyUsername"`
	ProxyPassword       string    `json:"proxyPassword"`
	UserDisconnected    bool      `json:"userDisconnected"`
	HighlightRegex      string    `json:"highlightRegex"`
	IgnoreList          []any     `json:"ignoreList"`
}

// UserConfig represents the full user configuration
type UserConfig struct {
	Log            bool                   `json:"log"`
	AwayMessage    string                 `json:"awayMessage"`
	ClientSettings map[string]interface{} `json:"clientSettings"`
	Networks       []Network              `json:"networks"`
}

// DefaultUserConfig returns a default configuration for new users
func DefaultUserConfig() *UserConfig {
	return &UserConfig{
		Log:            false,
		AwayMessage:    "",
		ClientSettings: make(map[string]interface{}),
		Networks:       []Network{},
	}
}
