// Package activity defines the safe operational metadata retained by the
// gateway and streamed to signed-in administrators. Raw device bytes belong only in
// device_events and are intentionally not represented here.
package activity

import "time"

const (
	KindAdminLogin                        = "admin.login"
	KindAdminLogout                       = "admin.logout"
	KindAdminISAPICommandQueued           = "admin.isapi_command_queued"
	KindAdminAccessEventSyncQueued        = "admin.access_event_sync_queued"
	KindPushSDKAuthInfo                   = "pushsdk.auth_info"
	KindPushSDKLogin                      = "pushsdk.login"
	KindPushSDKLogout                     = "pushsdk.logout"
	KindPushSDKSessionResumed             = "pushsdk.session_resumed"
	KindPushSDKSessionExpired             = "pushsdk.session_expired"
	KindPushSDKRejected                   = "pushsdk.rejected"
	KindPushSDKCommandSent                = "pushsdk.command_sent"
	KindPushSDKCommandCompleted           = "pushsdk.command_completed"
	KindPushSDKCommandExpired             = "pushsdk.command_expired"
	KindDeviceEventPersisted              = "device.event_persisted"
	KindDeviceEventDuplicate              = "device.event_duplicate"
	KindDeviceAccessEventSyncStarted      = "device.access_event_sync_started"
	KindDeviceAccessEventSyncPageCaptured = "device.access_event_sync_page_captured"
	KindDeviceAccessEventSyncCompleted    = "device.access_event_sync_completed"
	KindDeviceAccessEventSyncFailed       = "device.access_event_sync_failed"
)

type Event struct {
	ID       int64          `json:"id,omitempty"`
	At       time.Time      `json:"at"`
	Kind     string         `json:"kind"`
	Terminal string         `json:"terminal,omitempty"`
	Message  string         `json:"message"`
	Fields   map[string]any `json:"fields,omitempty"`
}
