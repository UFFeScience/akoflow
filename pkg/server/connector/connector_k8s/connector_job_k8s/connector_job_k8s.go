package connector_job_k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/entities/k8s_job_entity"
	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorJobK8s struct {
	client  *http.Client
	runtime *runtime_entity.Runtime
}

type IConnectorJob interface {
	ListJobs()
	ApplyJob(namespace string, job k8s_job_entity.K8sJob) interface{}
	GetJob(namespace string, jobName string) (ResponseGetJob, error)
	DeleteJob(jobName string, namespace string) error
}

var ErrJobNotFound = errors.New("job not found")

func New(runtime *runtime_entity.Runtime) IConnectorJob {
	return &ConnectorJobK8s{
		client:  newClient(),
		runtime: runtime,
	}
}
func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

func (c ConnectorJobK8s) ListJobs() {
	//TODO implement me
	panic("implement me")
}

func (c *ConnectorJobK8s) ApplyJob(namespace string, job k8s_job_entity.K8sJob) interface{} {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	body, _ := json.Marshal(&job)
	fmt.Println(string(body))

	req, _ := http.NewRequest("POST", "https://"+host+"/apis/batch/v1/namespaces/"+namespace+"/jobs", bytes.NewBuffer(body))

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)

	if err != nil {
		return nil
	}

	if resp.StatusCode != 201 {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		println("Error creating job: ", body.String())
		return nil
	}

	defer resp.Body.Close()

	var result interface{}
	println(resp.StatusCode)
	println(string(body))
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return nil
	}

	return result

}

type ResponseGetJob struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		Uid               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		Generation        int       `json:"generation"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Labels            struct {
			ControllerUid string `json:"controller-uid"`
			JobName       string `json:"job-name"`
		} `json:"labels"`
		Annotations struct {
			BatchKubernetesIoJobTracking string `json:"batch.kubernetes.io/job-tracking"`
		} `json:"annotations"`
		ManagedFields []struct {
			Manager    string    `json:"manager"`
			Operation  string    `json:"operation"`
			ApiVersion string    `json:"apiVersion"`
			Time       time.Time `json:"time"`
			FieldsType string    `json:"fieldsType"`
			FieldsV1   struct {
				FSpec struct {
					FBackoffLimit struct {
					} `json:"f:backoffLimit"`
					FCompletionMode struct {
					} `json:"f:completionMode"`
					FCompletions struct {
					} `json:"f:completions"`
					FParallelism struct {
					} `json:"f:parallelism"`
					FSuspend struct {
					} `json:"f:suspend"`
					FTemplate struct {
						FSpec struct {
							FContainers struct {
								KNameActivity077 struct {
									Field1 struct {
									} `json:"."`
									FCommand struct {
									} `json:"f:command"`
									FImage struct {
									} `json:"f:image"`
									FImagePullPolicy struct {
									} `json:"f:imagePullPolicy"`
									FName struct {
									} `json:"f:name"`
									FResources struct {
									} `json:"f:resources"`
									FTerminationMessagePath struct {
									} `json:"f:terminationMessagePath"`
									FTerminationMessagePolicy struct {
									} `json:"f:terminationMessagePolicy"`
								}
							} `json:"f:containers"`
							FDnsPolicy struct {
							} `json:"f:dnsPolicy"`
							FRestartPolicy struct {
							} `json:"f:restartPolicy"`
							FSchedulerName struct {
							} `json:"f:schedulerName"`
							FSecurityContext struct {
							} `json:"f:securityContext"`
							FTerminationGracePeriodSeconds struct {
							} `json:"f:terminationGracePeriodSeconds"`
						} `json:"f:spec"`
					} `json:"f:template"`
				} `json:"f:spec,omitempty"`
				FStatus struct {
					FCompletionTime struct {
					} `json:"f:completionTime"`
					FConditions struct {
					} `json:"f:conditions"`
					FReady struct {
					} `json:"f:ready"`
					FStartTime struct {
					} `json:"f:startTime"`
					FSucceeded struct {
					} `json:"f:succeeded"`
					FUncountedTerminatedPods struct {
					} `json:"f:uncountedTerminatedPods"`
				} `json:"f:status,omitempty"`
			} `json:"fieldsV1"`
			Subresource string `json:"subresource,omitempty"`
		} `json:"managedFields"`
	} `json:"metadata"`
	Spec struct {
		Parallelism  int `json:"parallelism"`
		Completions  int `json:"completions"`
		BackoffLimit int `json:"backoffLimit"`
		Selector     struct {
			MatchLabels struct {
				ControllerUid string `json:"controller-uid"`
			} `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
			Metadata struct {
				CreationTimestamp interface{} `json:"creationTimestamp"`
				Labels            struct {
					ControllerUid string `json:"controller-uid"`
					JobName       string `json:"job-name"`
				} `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				Containers []struct {
					Name      string   `json:"name"`
					Image     string   `json:"image"`
					Command   []string `json:"command"`
					Resources struct {
					} `json:"resources"`
					TerminationMessagePath   string `json:"terminationMessagePath"`
					TerminationMessagePolicy string `json:"terminationMessagePolicy"`
					ImagePullPolicy          string `json:"imagePullPolicy"`
				} `json:"containers"`
				RestartPolicy                 string `json:"restartPolicy"`
				TerminationGracePeriodSeconds int    `json:"terminationGracePeriodSeconds"`
				DnsPolicy                     string `json:"dnsPolicy"`
				SecurityContext               struct {
				} `json:"securityContext"`
				SchedulerName string `json:"schedulerName"`
			} `json:"spec"`
		} `json:"template"`
		CompletionMode string `json:"completionMode"`
		Suspend        bool   `json:"suspend"`
	} `json:"spec"`
	Status struct {
		Conditions []struct {
			Type               string    `json:"type"`
			Status             string    `json:"status"`
			LastProbeTime      time.Time `json:"lastProbeTime"`
			LastTransitionTime time.Time `json:"lastTransitionTime"`
		} `json:"conditions"`
		StartTime               time.Time `json:"startTime"`
		CompletionTime          time.Time `json:"completionTime"`
		Succeeded               int       `json:"succeeded"`
		UncountedTerminatedPods struct {
		} `json:"uncountedTerminatedPods"`
		Ready  int `json:"ready"`
		Active int `json:"active"`
		Failed int `json:"failed"`
	} `json:"status"`
}

type ResponseGetJob404 struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
	} `json:"metadata"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Details struct {
		Name  string `json:"name"`
		Group string `json:"group"`
		Kind  string `json:"kind"`
	} `json:"details"`
	Code int `json:"code"`
}

func (c *ConnectorJobK8s) GetJob(namespace string, jobName string) (ResponseGetJob, error) {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, _ := http.NewRequest("GET", "https://"+host+"/apis/batch/v1/namespaces/"+namespace+"/jobs/"+jobName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if resp == nil {
		return ResponseGetJob{}, errors.New("error getting job")
	}

	if resp.StatusCode == 404 {
		return ResponseGetJob{}, ErrJobNotFound
	}

	if err != nil {
		return ResponseGetJob{}, err
	}

	defer resp.Body.Close()

	var result ResponseGetJob
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseGetJob{}, err
	}

	return result, nil
}

func (c *ConnectorJobK8s) DeleteJob(jobName string, namespace string) error {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, _ := http.NewRequest("DELETE", "https://"+host+"/apis/batch/v1/namespaces/"+namespace+"/jobs/"+jobName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("error deleting job")
	}

	return nil
}
