package entities

import "time"

type Bucket struct {
	Name            string            `json:"name"`
	CreationDate    time.Time         `json:"creationDate"`
	NumberOfObjects int               `json:"numberOfObjects"`
	Size            int               `json:"size"`
	Objects         map[string]Object `json:"objects"`
}
