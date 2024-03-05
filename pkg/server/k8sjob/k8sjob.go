package k8sjob

type K8sJob struct {
	ApiVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					Name    string   `yaml:"name"`
					Image   string   `yaml:"image"`
					Command []string `yaml:"command"`
				} `yaml:"containers"`
				RestartPolicy string `yaml:"restartPolicy"`
				BackoffLimit  int    `yaml:"backoffLimit"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// docker run --rm alpine:latest bin/sh -c 'echo ZWNobyAiSGVsbG8gV29ybGQiCnNsZWVwIDUKZWNobyAiSGVsbG8gV29ybGQgQWdhaW4iCnNsZWVwIDUKZWNobyAiSGVsbG8gV29ybGQgT25lIE1vcmUgVGltZSI=| base64 -d| sh'
