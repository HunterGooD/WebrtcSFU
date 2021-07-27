package model

type Package struct {
	Head Head `json:"head"`
	Body Body `json:"body,omitempty"`
}

type Head struct {
	Event    string `json:"event"`
	UserID   string `json:"user_id,omitempty"`
	PubID    string `json:"pub_id,omitempty"`
	UserName string `json:"user_name,omitempty"`
}

type Body struct {
	SDP  string `json:"sdp,omitempty"`
	Data string `json:"data,omitempty"`
}
