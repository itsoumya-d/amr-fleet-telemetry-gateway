package telemetry

import "time"

type OperationalState string

const (
	StateIdle      OperationalState = "IDLE"
	StateCleaning  OperationalState = "CLEANING"
	StateMapping   OperationalState = "MAPPING"
	StateDocked    OperationalState = "DOCKED"
	StateFault     OperationalState = "FAULT"
	StateOffline   OperationalState = "OFFLINE"
)

type RobotPose struct {
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Heading float64 `json:"heading"`
}

type TelemetryPing struct {
	RobotID     string           `json:"robot_id"`
	Timestamp   time.Time        `json:"timestamp"`
	BatteryPct  float64          `json:"battery_pct"`
	State       OperationalState `json:"state"`
	Pose        RobotPose        `json:"pose"`
	ErrorCodes  []string         `json:"error_codes"`
}

type CommandType string

const (
	CmdReturnToDock CommandType = "RETURN_TO_DOCK"
	CmdPause        CommandType = "PAUSE_MISSION"
	CmdResume       CommandType = "RESUME_MISSION"
	CmdEStop        CommandType = "ESTOP"
)

type Command struct {
	ID       string      `json:"id"`
	RobotID  string      `json:"robot_id"`
	Type     CommandType `json:"type"`
	IssuedAt time.Time   `json:"issued_at"`
	SeqNum   uint64      `json:"seq_num"`
	Acked    bool        `json:"acked"`
}

type Alert struct {
	RobotID   string    `json:"robot_id"`
	Severity  string    `json:"severity"` // INFO, WARNING, CRITICAL
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type RobotRecord struct {
	RobotID      string           `json:"robot_id"`
	LastSeen     time.Time        `json:"last_seen"`
	BatteryPct   float64          `json:"battery_pct"`
	State        OperationalState `json:"state"`
	Pose         RobotPose        `json:"pose"`
	ActiveAlerts []string         `json:"active_alerts"`
}
