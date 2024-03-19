package connector_pvc_k8s

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type ConnectorPvcK8s struct {
	client *http.Client
}

func New() IConnectorPvc {
	return &ConnectorPvcK8s{
		client: newClient(),
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

type IConnectorPvc interface {
	ListPvcs()
	CreatePersistentVolumeClain(name string, namespace string, storageSize string, storageClassName string) (ResponseCreatePersistentVolumeClain, error)
	GetPersistentVolumeClain(name string, namespace string) (ResponseGetPersistentVolumeClain, error)
}

func (c ConnectorPvcK8s) ListPvcs() {
	//TODO implement me
	panic("implement me")
}

type PersistentVolumeClaim struct {
	ApiVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		AccessModes []string `json:"accessModes"`
		Resources   struct {
			Requests struct {
				Storage string `json:"storage"`
			} `json:"requests"`
		} `json:"resources"`
		StorageClassName string `json:"storageClassName"`
	} `json:"spec"`
}

type ResponseCreatePersistentVolumeClain struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		Uid               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Finalizers        []string  `json:"finalizers"`
		ManagedFields     []struct {
			Manager    string    `json:"manager"`
			Operation  string    `json:"operation"`
			ApiVersion string    `json:"apiVersion"`
			Time       time.Time `json:"time"`
			FieldsType string    `json:"fieldsType"`
			FieldsV1   struct {
				FSpec struct {
					FAccessModes struct {
					} `json:"f:accessModes"`
					FResources struct {
						FRequests struct {
							Field1 struct {
							} `json:"."`
							FStorage struct {
							} `json:"f:storage"`
						} `json:"f:requests"`
					} `json:"f:resources"`
					FStorageClassName struct {
					} `json:"f:storageClassName"`
					FVolumeMode struct {
					} `json:"f:volumeMode"`
				} `json:"f:spec"`
			} `json:"fieldsV1"`
		} `json:"managedFields"`
	} `json:"metadata"`
	Spec struct {
		AccessModes []string `json:"accessModes"`
		Resources   struct {
			Requests struct {
				Storage string `json:"storage"`
			} `json:"requests"`
		} `json:"resources"`
		StorageClassName string `json:"storageClassName"`
		VolumeMode       string `json:"volumeMode"`
	} `json:"spec"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

func (c *ConnectorPvcK8s) CreatePersistentVolumeClain(name string, namespace string, storageSize string, storageClassName string) (ResponseCreatePersistentVolumeClain, error) {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	payload := PersistentVolumeClaim{}

	payload.ApiVersion = "v1"
	payload.Kind = "PersistentVolumeClaim"
	payload.Metadata.Name = name
	payload.Metadata.Namespace = namespace
	payload.Spec.AccessModes = []string{"ReadWriteOnce"}
	payload.Spec.Resources.Requests.Storage = storageSize
	payload.Spec.StorageClassName = storageClassName

	body, _ := json.Marshal(&payload)
	fmt.Println(string(body))

	req, _ := http.NewRequest("POST", "https://"+host+"/api/v1/namespaces/"+namespace+"/persistentvolumeclaims", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)

	if err != nil {
		return ResponseCreatePersistentVolumeClain{}, err
	}

	defer resp.Body.Close()

	var result ResponseCreatePersistentVolumeClain
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseCreatePersistentVolumeClain{}, err
	}

	return result, nil

}

type ResponseGetPersistentVolumeClain struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		Uid               string    `json:"uid"`
		ResourceVersion   string    `json:"resourceVersion"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Annotations       struct {
			PvKubernetesIoBindCompleted              string `json:"pv.kubernetes.io/bind-completed"`
			PvKubernetesIoBoundByController          string `json:"pv.kubernetes.io/bound-by-controller"`
			VolumeBetaKubernetesIoStorageProvisioner string `json:"volume.beta.kubernetes.io/storage-provisioner"`
			VolumeKubernetesIoStorageProvisioner     string `json:"volume.kubernetes.io/storage-provisioner"`
		} `json:"annotations"`
		Finalizers    []string `json:"finalizers"`
		ManagedFields []struct {
			Manager    string    `json:"manager"`
			Operation  string    `json:"operation"`
			ApiVersion string    `json:"apiVersion"`
			Time       time.Time `json:"time"`
			FieldsType string    `json:"fieldsType"`
			FieldsV1   struct {
				FSpec struct {
					FAccessModes struct {
					} `json:"f:accessModes,omitempty"`
					FResources struct {
						FRequests struct {
							Field1 struct {
							} `json:"."`
							FStorage struct {
							} `json:"f:storage"`
						} `json:"f:requests"`
					} `json:"f:resources,omitempty"`
					FStorageClassName struct {
					} `json:"f:storageClassName,omitempty"`
					FVolumeMode struct {
					} `json:"f:volumeMode,omitempty"`
					FVolumeName struct {
					} `json:"f:volumeName,omitempty"`
				} `json:"f:spec,omitempty"`
				FMetadata struct {
					FAnnotations struct {
						Field1 struct {
						} `json:"."`
						FPvKubernetesIoBindCompleted struct {
						} `json:"f:pv.kubernetes.io/bind-completed"`
						FPvKubernetesIoBoundByController struct {
						} `json:"f:pv.kubernetes.io/bound-by-controller"`
						FVolumeBetaKubernetesIoStorageProvisioner struct {
						} `json:"f:volume.beta.kubernetes.io/storage-provisioner"`
						FVolumeKubernetesIoStorageProvisioner struct {
						} `json:"f:volume.kubernetes.io/storage-provisioner"`
					} `json:"f:annotations"`
				} `json:"f:metadata,omitempty"`
				FStatus struct {
					FAccessModes struct {
					} `json:"f:accessModes"`
					FCapacity struct {
						Field1 struct {
						} `json:"."`
						FStorage struct {
						} `json:"f:storage"`
					} `json:"f:capacity"`
					FPhase struct {
					} `json:"f:phase"`
				} `json:"f:status,omitempty"`
			} `json:"fieldsV1"`
			Subresource string `json:"subresource,omitempty"`
		} `json:"managedFields"`
	} `json:"metadata"`
	Spec struct {
		AccessModes []string `json:"accessModes"`
		Resources   struct {
			Requests struct {
				Storage string `json:"storage"`
			} `json:"requests"`
		} `json:"resources"`
		VolumeName       string `json:"volumeName"`
		StorageClassName string `json:"storageClassName"`
		VolumeMode       string `json:"volumeMode"`
	} `json:"spec"`
	Status struct {
		Phase       string   `json:"phase"`
		AccessModes []string `json:"accessModes"`
		Capacity    struct {
			Storage string `json:"storage"`
		} `json:"capacity"`
	} `json:"status"`
}

func (c *ConnectorPvcK8s) GetPersistentVolumeClain(name string, namespace string) (ResponseGetPersistentVolumeClain, error) {
	token := os.Getenv("K8S_API_SERVER_TOKEN")
	host := os.Getenv("K8S_API_SERVER_HOST")

	req, _ := http.NewRequest("GET", "https://"+host+"/api/v1/namespaces/"+namespace+"/persistentvolumeclaims/"+name, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.client.Do(req)

	if err != nil {
		return ResponseGetPersistentVolumeClain{}, err
	}

	defer resp.Body.Close()

	var result ResponseGetPersistentVolumeClain
	err = json.NewDecoder(resp.Body).Decode(&result)

	if err != nil {
		return ResponseGetPersistentVolumeClain{}, err
	}

	return result, nil

}
