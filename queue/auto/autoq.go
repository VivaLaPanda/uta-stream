package auto

type Queue struct {
}

func NewQueue() *Queue {
	return &Queue{}
}

func (*Queue) Vpop() string {
	return ""
}
