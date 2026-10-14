package pvs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coder/websocket"
)

// Frame is one live data message. Params are in kW, kWh, and 0-1 for soc;
// Time is Unix seconds.
type Frame struct {
	Time   int64
	Params map[string]float64
}

// Stream reads the live data websocket at url until the connection fails or
// ctx ends, calling emit for each power frame. A connection that is not up,
// or sends nothing, within stall has failed, as has one sending a frame that
// does not decode. Stream returns why the connection ended.
func Stream(ctx context.Context, url string, stall time.Duration, emit func(Frame)) error {
	dialCtx, cancel := context.WithTimeout(ctx, stall)
	conn, _, err := websocket.Dial(dialCtx, url, nil)
	cancel()
	if err != nil {
		return err
	}
	defer conn.CloseNow()
	for {
		readCtx, cancel := context.WithTimeout(ctx, stall)
		_, data, err := conn.Read(readCtx)
		cancel()
		if err != nil {
			return err
		}
		var msg struct {
			Notification string          `json:"notification"`
			Params       json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			return fmt.Errorf("decoding a live frame: %w", err)
		}
		if msg.Notification != powerNotification {
			continue
		}
		var params map[string]float64
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return fmt.Errorf("decoding a live frame: %w", err)
		}
		t, ok := params[timeParam]
		if !ok {
			continue
		}
		delete(params, timeParam)
		emit(Frame{Time: int64(t), Params: params})
	}
}
