package models

type SignupMode string

const (
	SignupModePublic   SignupMode = "public"
	SignupModeApproval SignupMode = "approval"
	SignupModeInvite   SignupMode = "invite"
)

type InstanceSettings struct {
	SignupMode  SignupMode `json:"signup_mode"`
	MOTD        string     `json:"motd"`
	Domain      string     `json:"domain"`
	NetworkName string     `json:"network_name"`
}

// DefaultInstanceSettings returns default settings
func DefaultInstanceSettings() *InstanceSettings {
	return &InstanceSettings{
		SignupMode:  SignupModePublic,
		MOTD:        "Welcome to Zubr! Please be respectful.",
		Domain:      "localhost",
		NetworkName: "zubr",
	}
}
