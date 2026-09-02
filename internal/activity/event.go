// Package activity defines the safe operational metadata retained by the
// gateway and streamed to signed-in operators. Raw device bytes belong only in
// device_events and are intentionally not represented here.
package activity

import "time"

const (
	KindAdminLogin            = "admin.login"
	KindAdminLogout           = "admin.logout"
	KindPushSDKAuthInfo       = "pushsdk.auth_info"
	KindPushSDKLogin          = "pushsdk.login"
	KindPushSDKLogout         = "pushsdk.logout"
	KindPushSDKSessionResumed = "pushsdk.session_resumed"
	KindPushSDKRejected       = "pushsdk.rejected"
	KindDeviceEventPersisted  = "device.event_persisted"
	KindDeviceEventDuplicate  = "device.event_duplicate"
)

type Event struct {
	ID       int64          `json:"id,omitempty"`
	At       time.Time      `json:"at"`
	Kind     string         `json:"kind"`
	Terminal string         `json:"terminal,omitempty"`
	Message  string         `json:"message"`
	Fields   map[string]any `json:"fields,omitempty"`
}
