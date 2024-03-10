package connector

import (
	"errors"
	"time"
)

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
								} `json:"k:{"name":"activity-077"}"`
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
	} `json:"status"`
}

type ResponseGetJobByPod struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
		ResourceVersion string `json:"resourceVersion"`
	} `json:"metadata"`
	Items []struct {
		Metadata struct {
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
		} `json:"metadata"`
		Spec struct {
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
	} `json:"items"`
}

func (c ResponseGetJobByPod) GetPodName() (string, error) {
	if len(c.Items) == 0 {
		// return err
		return "", errors.New("no pod found")
	}

	return c.Items[0].Metadata.Name, nil
}

type ResponseGetPodMetrics struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Metadata   struct {
		Name              string    `json:"name"`
		Namespace         string    `json:"namespace"`
		CreationTimestamp time.Time `json:"creationTimestamp"`
		Labels            struct {
			App             string `json:"app"`
			PodTemplateHash string `json:"pod-template-hash"`
		} `json:"labels"`
	} `json:"metadata"`
	Timestamp  time.Time `json:"timestamp"`
	Window     string    `json:"window"`
	Containers []struct {
		Name  string `json:"name"`
		Usage struct {
			Cpu    string `json:"cpu"`
			Memory string `json:"memory"`
		} `json:"usage"`
	} `json:"containers"`
}

type Metrics struct {
	ActivityId *int
	Window     string
	Timestamp  string
	Cpu        string `json:"cpu"`
	Memory     string `json:"memory"`
}

func (c ResponseGetPodMetrics) GetMetrics() (Metrics, error) {
	metrics := Metrics{}

	if len(c.Containers) == 0 {
		return metrics, errors.New("no metrics found")
	}

	metrics.Window = c.Window
	metrics.Timestamp = c.Timestamp.String()

	metrics.Cpu = c.Containers[0].Usage.Cpu
	metrics.Memory = c.Containers[0].Usage.Memory

	return metrics, nil
}
