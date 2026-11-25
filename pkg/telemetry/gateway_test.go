package telemetry

import (
	"sync"
	"testing"
	"time"
)

func TestConcurrentTelemetryIngestion(t *testing.T) {
	gw := NewGateway(5*time.Second, 20.0)
	now := time.Now()

	var wg sync.WaitGroup
	numRobots := 10
	pingsPerRobot := 50

	for i := 0; i < numRobots; i++ {
		wg.Add(1)
		go func(robotIdx int) {
			defer wg.Done()
			robotID := "amr-" + string(rune('A'+robotIdx))
			for p := 0; p < pingsPerRobot; p++ {
				ping := TelemetryPing{
					RobotID:    robotID,
					Timestamp:  now.Add(time.Duration(p) * time.Millisecond),
					BatteryPct: 80.0 - float64(p)*0.1,
					State:      StateCleaning,
					Pose: RobotPose{
						X:       float64(p) * 0.5,
						Y:       float64(p) * 0.2,
						Heading: 1.57,
					},
				}
				_, err := gw.Ingest(ping)
				if err != nil {
					t.Errorf("unexpected error ingesting ping: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	summary := gw.FleetSummary()
	if summary[StateCleaning] != numRobots {
		t.Fatalf("expected %d cleaning robots, got %d", numRobots, summary[StateCleaning])
	}
}

func TestLowBatteryAutoDocking(t *testing.T) {
	gw := NewGateway(10*time.Second, 15.0)
	now := time.Now()

	ping := TelemetryPing{
		RobotID:    "amr-cleaner-01",
		Timestamp:  now,
		BatteryPct: 12.0, // below 15% threshold
		State:      StateCleaning,
		Pose:       RobotPose{X: 10, Y: 20, Heading: 0},
	}

	alerts, err := gw.Ingest(ping)
	if err != nil {
		t.Fatalf("unexpected ingest error: %v", err)
	}
	if len(alerts) == 0 {
		t.Fatal("expected low battery alert")
	}

	pending := gw.GetPendingCommands("amr-cleaner-01")
	if len(pending) != 1 {
		t.Fatalf("expected 1 auto-dock command, got %d", len(pending))
	}
	if pending[0].Type != CmdReturnToDock {
		t.Errorf("expected RETURN_TO_DOCK, got %s", pending[0].Type)
	}

	// Ack the command
	err = gw.AckCommand("amr-cleaner-01", pending[0].ID)
	if err != nil {
		t.Fatalf("failed to ack command: %v", err)
	}

	pendingAfter := gw.GetPendingCommands("amr-cleaner-01")
	if len(pendingAfter) != 0 {
		t.Errorf("expected 0 pending after ack, got %d", len(pendingAfter))
	}
}

func TestLivenessHeartbeatExpiration(t *testing.T) {
	gw := NewGateway(2*time.Second, 20.0)
	baseTime := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

	// Robot 1 pings at baseTime
	gw.Ingest(TelemetryPing{
		RobotID:    "amr-01",
		Timestamp:  baseTime,
		BatteryPct: 90,
		State:      StateIdle,
	})

	// Robot 2 pings at baseTime + 1.5s
	gw.Ingest(TelemetryPing{
		RobotID:    "amr-02",
		Timestamp:  baseTime.Add(1500 * time.Millisecond),
		BatteryPct: 95,
		State:      StateCleaning,
	})

	// Scan liveness at baseTime + 2.5s (amr-01 elapsed 2.5s > 2.0s timeout; amr-02 elapsed 1.0s < 2.0s)
	timedOut := gw.ScanLiveness(baseTime.Add(2500 * time.Millisecond))
	if len(timedOut) != 1 || timedOut[0] != "amr-01" {
		t.Fatalf("expected amr-01 to time out, got %v", timedOut)
	}

	rec, ok := gw.GetRobotState("amr-01")
	if !ok || rec.State != StateOffline {
		t.Errorf("expected amr-01 to be OFFLINE, got %s", rec.State)
	}

	// Dispatch command to offline robot should fail
	_, err := gw.DispatchCommand("amr-01", CmdPause)
	if err != ErrRobotOffline {
		t.Errorf("expected ErrRobotOffline, got %v", err)
	}
}

func TestHardwareFaultHandling(t *testing.T) {
	gw := NewGateway(5*time.Second, 20.0)
	now := time.Now()

	ping := TelemetryPing{
		RobotID:    "amr-faulty-09",
		Timestamp:  now,
		BatteryPct: 75,
		State:      StateCleaning,
		ErrorCodes: []string{"LIDAR_OBSTRUCTED", "MOTOR_OVERHEAT"},
	}

	alerts, err := gw.Ingest(ping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 || alerts[0].Severity != "CRITICAL" {
		t.Fatalf("expected CRITICAL alert, got %v", alerts)
	}

	rec, _ := gw.GetRobotState("amr-faulty-09")
	if rec.State != StateFault {
		t.Errorf("expected FAULT state, got %s", rec.State)
	}
}
