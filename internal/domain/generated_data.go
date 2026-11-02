package domain

type GeneratedDataItem struct {
	ID           string `json:"id"`
	Key          string `json:"key"`
	RunID        string `json:"runId"`
	LogicalName  string `json:"logicalName"`
	RelativePath string `json:"relativePath"`
	SizeBytes    int64  `json:"sizeBytes"`
	Locations    int    `json:"locations"`
}

type GeneratedDataPage struct {
	Items    []GeneratedDataItem `json:"items"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
	Total    int                 `json:"total"`
	HasNext  bool                `json:"hasNext"`
}
