package commonservices

import (
	"testing"
	"time"
)

func TestUserPresenceLifecycle(t *testing.T) {
	appKey := "test_app_presence"
	userId := "user_presence_1"

	// 1. Initial state without setting should be offline
	initPresence := GetUserPresence(appKey, userId)
	if initPresence.Status != "offline" {
		t.Fatalf("expected offline for unset user, got %s", initPresence.Status)
	}

	// 2. Set online with custom status
	UpdateUserPresence(appKey, userId, "busy", "In an executive meeting", "Web", 3600)
	current := GetUserPresence(appKey, userId)
	if current.Status != "busy" || current.CustomStatus != "In an executive meeting" || current.Platform != "Web" {
		t.Fatalf("unexpected presence state: %+v", current)
	}

	// 3. Test expiry logic with 1-second timeout
	UpdateUserPresence(appKey, userId, "dnd", "Do Not Disturb for 1 second", "iOS", 1)
	time.Sleep(1100 * time.Millisecond)
	expired := GetUserPresence(appKey, userId)
	if expired.CustomStatus != "" {
		t.Fatalf("expected custom status to expire and clear, got: %s", expired.CustomStatus)
	}
}

