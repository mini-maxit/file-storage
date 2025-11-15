package entities


type Bucket struct {
	Name            string            `json:"name"`
	NumberOfObjects int               `json:"numberOfObjects"`
	Objects         map[string]Object `json:"objects,omitempty"`
}
