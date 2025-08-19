package queue

type Message struct {
	Key     string
	Body    []byte
	Headers map[string]string
}
