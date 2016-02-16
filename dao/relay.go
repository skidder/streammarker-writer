package dao

type Relay struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	State     string `json:"state"`
}

func (r *Relay) isActive() bool {
	return (r.State == "active")
}
