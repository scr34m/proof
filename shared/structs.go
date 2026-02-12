package shared

type QueuePacket struct {
	Body      []byte `json:"body"`
	Protocol  string `json:"protocol"`
	ProjectId string `json:"project_id"`
}
