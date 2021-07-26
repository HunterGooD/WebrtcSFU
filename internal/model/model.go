package model

type Package struct {
	Head Head `json:"head"`
	Body Body `json:"body"`
}

type Head struct {
	Event  string `json:"event"`
	UserID string `json:"user_id"`
	PubID  string `json:"pub_id"`
}

type Body struct {
	Data string `json:"data"`
}
