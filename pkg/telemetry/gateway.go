package telemetry

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrRobotNotFound = errors.New("robot not found")
	ErrRobotOffline  = errors.New("cannot dispatch command to offline robot")
	ErrCommandNotFound = errors.New("command not found")
)

type Gateway struct {
	mu                  sync.RWMutex
	fleet               map[string]*RobotRecord
	commandQueues       map[string][]*Command
	alerts              []Alert
	heartbeatTimeout    time.Duration
	lowBatteryThreshold float64
	nextSeq             uint64
}

func NewGateway(heartbeatTimeout time.Duration, lowBatteryThreshold float64) *Gateway {
	return &Gateway{
		fleet:               make(map[string]*RobotRecord),
		commandQueues:       make(map[string][]*Command),
		alerts:              make([]Alert, 0),
		heartbeatTimeout:    heartbeatTimeout,
		lowBatteryThreshold: lowBatteryThreshold,
		nextSeq:             1,
	}
}

func (g *Gateway) Ingest(ping TelemetryPing) ([]Alert, error) {
	if ping.RobotID == "" {
		return nil, errors.New("robot_id cannot be empty")
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	rec, exists := g.fleet[ping.RobotID]
	if !exists {
		rec = &RobotRecord{
			RobotID: ping.RobotID,
		}
		g.fleet[ping.RobotID] = rec
	}

	rec.LastSeen = ping.Timestamp
	rec.BatteryPct = ping.BatteryPct
	rec.State = ping.State
	rec.Pose = ping.Pose
	rec.ActiveAlerts = ping.ErrorCodes

	var newAlerts []Alert

	// 1. Critical hardware / sensor error alert
	if len(ping.ErrorCodes) > 0 {
		rec.State = StateFault
		alert := Alert{
			RobotID:   ping.RobotID,
			Severity:  "CRITICAL",
			Message:   fmt.Sprintf("Hardware fault detected: %v", ping.ErrorCodes),
			Timestamp: ping.Timestamp,
		}
		newAlerts = append(newAlerts, alert)
		g.alerts = append(g.alerts, alert)
	}

	// 2. Battery threshold evaluation
	if ping.BatteryPct < g.lowBatteryThreshold && ping.State != StateDocked {
		alert := Alert{
			RobotID:   ping.RobotID,
			Severity:  "WARNING",
			Message:   fmt.Sprintf("Low battery (%.1f%%); triggering auto-return to dock", ping.BatteryPct),
			Timestamp: ping.Timestamp,
		}
		newAlerts = append(newAlerts, alert)
		g.alerts = append(g.alerts, alert)

		// Auto-queue RETURN_TO_DOCK command if not already queued
		hasDockCmd := false
		for _, cmd := range g.commandQueues[ping.RobotID] {
			if cmd.Type == CmdReturnToDock && !cmd.Acked {
				hasDockCmd = true
				break
			}
		}
		if !hasDockCmd {
			g.nextSeq++
			cmd := &Command{
				ID:       fmt.Sprintf("cmd-autodock-%d", g.nextSeq),
				RobotID:  ping.RobotID,
				Type:     CmdReturnToDock,
				IssuedAt: ping.Timestamp,
				SeqNum:   g.nextSeq,
				Acked:    false,
			}
			g.commandQueues[ping.RobotID] = append(g.commandQueues[ping.RobotID], cmd)
		}
	}

	return newAlerts, nil
}

func (g *Gateway) ScanLiveness(now time.Time) []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	var timedOutRobots []string
	for id, rec := range g.fleet {
		if rec.State != StateOffline && now.Sub(rec.LastSeen) > g.heartbeatTimeout {
			rec.State = StateOffline
			timedOutRobots = append(timedOutRobots, id)
			alert := Alert{
				RobotID:   id,
				Severity:  "WARNING",
				Message:   fmt.Sprintf("Heartbeat timed out after %v", now.Sub(rec.LastSeen)),
				Timestamp: now,
			}
			g.alerts = append(g.alerts, alert)
		}
	}
	return timedOutRobots
}

func (g *Gateway) DispatchCommand(robotID string, cmdType CommandType) (*Command, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	rec, exists := g.fleet[robotID]
	if !exists {
		return nil, ErrRobotNotFound
	}
	if rec.State == StateOffline {
		return nil, ErrRobotOffline
	}

	g.nextSeq++
	cmd := &Command{
		ID:       fmt.Sprintf("cmd-%d", g.nextSeq),
		RobotID:  robotID,
		Type:     cmdType,
		IssuedAt: time.Now(),
		SeqNum:   g.nextSeq,
		Acked:    false,
	}

	g.commandQueues[robotID] = append(g.commandQueues[robotID], cmd)
	return cmd, nil
}

func (g *Gateway) AckCommand(robotID string, cmdID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	queue, exists := g.commandQueues[robotID]
	if !exists {
		return ErrCommandNotFound
	}

	for _, cmd := range queue {
		if cmd.ID == cmdID {
			cmd.Acked = true
			return nil
		}
	}
	return ErrCommandNotFound
}

func (g *Gateway) GetPendingCommands(robotID string) []*Command {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var pending []*Command
	for _, cmd := range g.commandQueues[robotID] {
		if !cmd.Acked {
			pending = append(pending, cmd)
		}
	}
	return pending
}

func (g *Gateway) GetRobotState(robotID string) (*RobotRecord, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	rec, exists := g.fleet[robotID]
	if !exists {
		return nil, false
	}
	// Return copy
	cp := *rec
	return &cp, true
}

func (g *Gateway) FleetSummary() map[OperationalState]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	summary := make(map[OperationalState]int)
	for _, rec := range g.fleet {
		summary[rec.State]++
	}
	return summary
}
