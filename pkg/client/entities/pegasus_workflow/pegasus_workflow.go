package pegasus_workflow

type PegasusWorkflow struct {
	XPegasus struct {
		APILang   string `yaml:"apiLang"`
		CreatedBy string `yaml:"createdBy"`
		CreatedOn string `yaml:"createdOn"`
	} `yaml:"x-pegasus"`
	Pegasus        string `yaml:"pegasus"`
	Name           string `yaml:"name"`
	ReplicaCatalog struct {
		Replicas []struct {
			Lfn  string `yaml:"lfn"`
			Pfns []struct {
				Site string `yaml:"site"`
				Pfn  string `yaml:"pfn"`
			} `yaml:"pfns"`
		} `yaml:"replicas"`
	} `yaml:"replicaCatalog"`
	TransformationCatalog struct {
		Transformations []struct {
			Name  string `yaml:"name"`
			Sites []struct {
				Name string `yaml:"name"`
				Pfn  string `yaml:"pfn"`
				Type string `yaml:"type"`
			} `yaml:"sites"`
			Profiles struct {
				Condor struct {
					RequestMemory string `yaml:"request_memory"`
				} `yaml:"condor"`
				Env struct {
					PATH string `yaml:"PATH"`
				} `yaml:"env"`
			} `yaml:"profiles,omitempty"`
			Requires []string `yaml:"requires,omitempty"`
		} `yaml:"transformations"`
	} `yaml:"transformationCatalog"`
	Jobs []struct {
		Type      string   `yaml:"type"`
		Name      string   `yaml:"name"`
		ID        string   `yaml:"id"`
		Arguments []string `yaml:"arguments"`
		Uses      []struct {
			Lfn             string `yaml:"lfn"`
			Type            string `yaml:"type"`
			StageOut        bool   `yaml:"stageOut,omitempty"`
			RegisterReplica bool   `yaml:"registerReplica,omitempty"`
		} `yaml:"uses"`
	} `yaml:"jobs"`
	JobDependencies []struct {
		ID       string   `yaml:"id"`
		Children []string `yaml:"children"`
	} `yaml:"jobDependencies"`
}
