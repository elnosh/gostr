package main

type Event struct {
	Id        string   `json:"id"`
	Pubkey    string   `json:"pubkey"`
	CreatedAt int64    `json:"created_at"`
	Kind      int      `json:"kind"`
	Tags      []string `json:"tags"`
	Content   string   `json:"content"`
	Sig       string   `json:"sig"`
}
