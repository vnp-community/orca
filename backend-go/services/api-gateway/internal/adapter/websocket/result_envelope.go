package websocket

type ResultEnvelope struct {
	ID      string
	Payload []byte
	Error   string
}
