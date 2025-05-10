package connector_pod_k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ovvesley/akoflow/pkg/server/entities/runtime_entity"
)

type ConnectorPodK8s struct {
	client  *http.Client
	runtime *runtime_entity.Runtime
}

type IConnectorPod interface {
	ListPods()
	GetPodByJob(namespace string, jobName string) (ResponseGetJobByPod, error)
	GetPodLogs(namespace string, podName string) (string, error)
	DeletePod(namespace string, podName string) error
}

func New(runtime *runtime_entity.Runtime) IConnectorPod {
	return &ConnectorPodK8s{
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

func (c ConnectorPodK8s) ListPods() {
	//TODO implement me
	panic("implement me")
}

type ResponseGetJobByPodItemMetadata struct {
	Name              string    `json:"name"`
	GenerateName      string    `json:"generateName"`
	Namespace         string    `json:"namespace"`
	Uid               string    `json:"uid"`
	ResourceVersion   string    `json:"resourceVersion"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
	Labels            struct {
		ControllerUid string `json:"controller-uid"`
		JobName       string `json:"job-name"`
	} `json:"labels"`
	OwnerReferences []struct {
		ApiVersion         string `json:"apiVersion"`
		Kind               string `json:"kind"`
		Name               string `json:"name"`
		Uid                string `json:"uid"`
		Controller         bool   `json:"controller"`
		BlockOwnerDeletion bool   `json:"blockOwnerDeletion"`
	} `json:"ownerReferences"`
	Finalizers    []string `json:"finalizers"`
	ManagedFields []struct {
		Manager    string    `json:"manager"`
		Operation  string    `json:"operation"`
		ApiVersion string    `json:"apiVersion"`
		Time       time.Time `json:"time"`
		FieldsType string    `json:"fieldsType"`
		FieldsV1   struct {
			FMetadata struct {
				FFinalizers struct {
					Field1 struct {
					} `json:"."`
					VBatchKubernetesIoJobTracking struct {
					} `json:"v:"batch.kubernetes.io/job-tracking""`
				} `json:"f:finalizers"`
				FGenerateName struct {
				} `json:"f:generateName"`
				FLabels struct {
					Field1 struct {
					} `json:"."`
					FControllerUid struct {
					} `json:"f:controller-uid"`
					FJobName struct {
					} `json:"f:job-name"`
				} `json:"f:labels"`
				FOwnerReferences struct {
					Field1 struct {
					} `json:"."`
					KUid6230C1C6182147FcBa622383D0D31D4F struct {
					} `json:"k:{"uid":"6230c1c6-1821-47fc-ba62-2383d0d31d4f"}"`
				} `json:"f:ownerReferences"`
			} `json:"f:metadata,omitempty"`
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
					} `json:"k:{"name":"activity-077"}"`
				} `json:"f:containers"`
				FDnsPolicy struct {
				} `json:"f:dnsPolicy"`
				FEnableServiceLinks struct {
				} `json:"f:enableServiceLinks"`
				FRestartPolicy struct {
				} `json:"f:restartPolicy"`
				FSchedulerName struct {
				} `json:"f:schedulerName"`
				FSecurityContext struct {
				} `json:"f:securityContext"`
				FTerminationGracePeriodSeconds struct {
				} `json:"f:terminationGracePeriodSeconds"`
			} `json:"f:spec,omitempty"`
			FStatus struct {
				FConditions struct {
					KTypeContainersReady struct {
						Field1 struct {
						} `json:"."`
						FLastProbeTime struct {
						} `json:"f:lastProbeTime"`
						FLastTransitionTime struct {
						} `json:"f:lastTransitionTime"`
						FStatus struct {
						} `json:"f:status"`
						FType struct {
						} `json:"f:type"`
					} `json:"k:{"type":"ContainersReady"}"`
					KTypeInitialized struct {
						Field1 struct {
						} `json:"."`
						FLastProbeTime struct {
						} `json:"f:lastProbeTime"`
						FLastTransitionTime struct {
						} `json:"f:lastTransitionTime"`
						FStatus struct {
						} `json:"f:status"`
						FType struct {
						} `json:"f:type"`
					} `json:"k:{"type":"Initialized"}"`
					KTypeReady struct {
						Field1 struct {
						} `json:"."`
						FLastProbeTime struct {
						} `json:"f:lastProbeTime"`
						FLastTransitionTime struct {
						} `json:"f:lastTransitionTime"`
						FStatus struct {
						} `json:"f:status"`
						FType struct {
						} `json:"f:type"`
					} `json:"k:{"type":"Ready"}"`
				} `json:"f:conditions"`
				FContainerStatuses struct {
				} `json:"f:containerStatuses"`
				FHostIP struct {
				} `json:"f:hostIP"`
				FPhase struct {
				} `json:"f:phase"`
				FPodIP struct {
				} `json:"f:podIP"`
				FPodIPs struct {
					Field1 struct {
					} `json:"."`
					KIp101081 struct {
						Field1 struct {
						} `json:"."`
						FIp struct {
						} `json:"f:ip"`
					} `json:"k:{"ip":"10.1.0.81"}"`
				} `json:"f:podIPs"`
				FStartTime struct {
				} `json:"f:startTime"`
			} `json:"f:status,omitempty"`
		} `json:"fieldsV1"`
		Subresource string `json:"subresource,omitempty"`
	} `json:"managedFields"`
}
type ResponseGetJobByPodItem struct {
	Metadata ResponseGetJobByPodItemMetadata `json:"metadata"`
	Spec     struct {
		Volumes []struct {
			Name      string `json:"name"`
			Projected struct {
				Sources []struct {
					ServiceAccountToken struct {
						ExpirationSeconds int    `json:"expirationSeconds"`
						Path              string `json:"path"`
					} `json:"serviceAccountToken,omitempty"`
					ConfigMap struct {
						Name  string `json:"name"`
						Items []struct {
							Key  string `json:"key"`
							Path string `json:"path"`
						} `json:"items"`
					} `json:"configMap,omitempty"`
					DownwardAPI struct {
						Items []struct {
							Path     string `json:"path"`
							FieldRef struct {
								ApiVersion string `json:"apiVersion"`
								FieldPath  string `json:"fieldPath"`
							} `json:"fieldRef"`
						} `json:"items"`
					} `json:"downwardAPI,omitempty"`
				} `json:"sources"`
				DefaultMode int `json:"defaultMode"`
			} `json:"projected"`
		} `json:"volumes"`
		Containers []struct {
			Name      string   `json:"name"`
			Image     string   `json:"image"`
			Command   []string `json:"command"`
			Resources struct {
			} `json:"resources"`
			VolumeMounts []struct {
				Name      string `json:"name"`
				ReadOnly  bool   `json:"readOnly"`
				MountPath string `json:"mountPath"`
			} `json:"volumeMounts"`
			TerminationMessagePath   string `json:"terminationMessagePath"`
			TerminationMessagePolicy string `json:"terminationMessagePolicy"`
			ImagePullPolicy          string `json:"imagePullPolicy"`
		} `json:"containers"`
		RestartPolicy                 string `json:"restartPolicy"`
		TerminationGracePeriodSeconds int    `json:"terminationGracePeriodSeconds"`
		DnsPolicy                     string `json:"dnsPolicy"`
		ServiceAccountName            string `json:"serviceAccountName"`
		ServiceAccount                string `json:"serviceAccount"`
		NodeName                      string `json:"nodeName"`
		SecurityContext               struct {
		} `json:"securityContext"`
		SchedulerName string `json:"schedulerName"`
		Tolerations   []struct {
			Key               string `json:"key"`
			Operator          string `json:"operator"`
			Effect            string `json:"effect"`
			TolerationSeconds int    `json:"tolerationSeconds"`
		} `json:"tolerations"`
		Priority           int    `json:"priority"`
		EnableServiceLinks bool   `json:"enableServiceLinks"`
		PreemptionPolicy   string `json:"preemptionPolicy"`
	} `json:"spec"`
	Status struct {
		Phase      string `json:"phase"`
		Conditions []struct {
			Type               string      `json:"type"`
			Status             string      `json:"status"`
			LastProbeTime      interface{} `json:"lastProbeTime"`
			LastTransitionTime time.Time   `json:"lastTransitionTime"`
		} `json:"conditions"`
		HostIP string `json:"hostIP"`
		PodIP  string `json:"podIP"`
		PodIPs []struct {
			Ip string `json:"ip"`
		} `json:"podIPs"`
		StartTime         time.Time `json:"startTime"`
		ContainerStatuses []struct {
			Name  string `json:"name"`
			State struct {
				Running struct {
					StartedAt time.Time `json:"startedAt"`
				} `json:"running"`
			} `json:"state"`
			LastState struct {
			} `json:"lastState"`
			Ready        bool   `json:"ready"`
			RestartCount int    `json:"restartCount"`
			Image        string `json:"image"`
			ImageID      string `json:"imageID"`
			ContainerID  string `json:"containerID"`
			Started      bool   `json:"started"`
		} `json:"containerStatuses"`
		QosClass string `json:"qosClass"`
	} `json:"status"`
}

type ResponseGetJobByPod struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []ResponseGetJobByPodItem `json:"items"`
}

func (c *ConnectorPodK8s) GetPodByJob(namespace string, jobName string) (ResponseGetJobByPod, error) {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, _ := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/"+namespace+"/pods?labelSelector=job-name="+jobName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return ResponseGetJobByPod{}, err
	}

	defer resp.Body.Close()

	var result ResponseGetJobByPod
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseGetJobByPod{}, err
	}

	return result, nil
}
func (c ResponseGetJobByPod) GetPodName() (string, error) {
	if len(c.Items) == 0 {
		// return err
		return "", errors.New("no pod found")
	}

	return c.Items[0].Metadata.Name, nil
}

func (c *ConnectorPodK8s) GetPodLogs(namespace string, podName string) (string, error) {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, _ := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/"+namespace+"/pods/"+podName+"/log", nil)

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	buf := new(bytes.Buffer)

	buf.ReadFrom(resp.Body)

	return buf.String(), nil
}

func (c *ConnectorPodK8s) DeletePod(namespace string, podName string) error {
	token := c.runtime.GetMetadataApiServerToken()
	host := c.runtime.GetMetadataApiServerHost()

	req, _ := http.NewRequest("DELETE", "https://"+host+"/api/v1/namespaces/"+namespace+"/pods/"+podName, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("Error deleting pod")
	}

	return nil
}
