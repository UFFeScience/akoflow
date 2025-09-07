package types_api

type ApiScheduleType struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`
	Code      string `json:"code"`
	Name      string `json:"name,omitempty"` // Optional field, not always present
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}
