package entities

import "time"

type Object struct {
	Key          string    `json:"key"`
	Size         int       `json:"size"`
	LastModified time.Time `json:"lastModified"`
	Type         string    `json:"type"`
}
