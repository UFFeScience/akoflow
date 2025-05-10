package types_api

type ApiRuntimeType struct {
	Name      string            `yaml:"name" json:"name"`
	Status    int               `yaml:"status" json:"status"`
	Metadata  map[string]string `yaml:"metadata" json:"metadata"`
	CreatedAt string            `yaml:"createdAt" json:"createdAt"`
	UpdatedAt string            `yaml:"updatedAt" json:"updatedAt"`
	DeletedAt string            `yaml:"deletedAt" json:"deletedAt"`
}
