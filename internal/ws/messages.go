package ws

// Message is the wrapper for all WebSocket messages
type Message struct {
	Type string      `json:"type"` // "key_event", "stats", "status"
	Data interface{} `json:"data"`
}

// KeyEventData represents a key operation event
type KeyEventData struct {
	Op  string `json:"op"`  // "set", "del", "expire", "expired", "rename_from", "rename_to"
	Key string `json:"key"`
}

// StatsData represents periodic stats updates
type StatsData struct {
	DBSize          int64 `json:"dbSize"`
	NotificationsOn bool  `json:"notificationsOn"`
}

// StatusData represents connection status information
type StatusData struct {
	Live bool   `json:"live"`          // true if keyspace notifications are enabled
	Msg  string `json:"msg,omitempty"` // optional message
}
