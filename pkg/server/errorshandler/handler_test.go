package errorshandler

import "testing"

func TestDetermineQueue(t *testing.T) {
	handler := &Handler{
		NonDefaultQueues: map[string]bool{
			"errors/custom": true,
		},
	}

	if got := handler.determineQueue("errors/custom"); got != "errors/custom" {
		t.Fatalf("expected custom queue, got %s", got)
	}
	if got := handler.determineQueue("errors/unknown"); got != DefaultQueueName {
		t.Fatalf("expected default queue, got %s", got)
	}
}

func TestGetTimeSeriesKey(t *testing.T) {
	got := getTimeSeriesKey("proj-1", "events-accepted", "hourly")
	want := "ts:project-events-accepted:proj-1:hourly"
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
