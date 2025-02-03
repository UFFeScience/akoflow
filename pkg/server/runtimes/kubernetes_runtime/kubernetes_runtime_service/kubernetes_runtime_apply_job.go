package kubernetes_runtime_service

import "github.com/ovvesley/akoflow/pkg/server/config"

func (r *KubernetesRuntimeService) ApplyJob(activityID int) {

	wfa, err := r.activityRepository.Find(activityID)
	wf, _ := r.workflowRepository.Find(wfa.WorkflowId)

	if err != nil {
		config.App().Logger.Infof("WORKER: Activity not found %d", activityID)
		return
	}

	modeService := ModeRunActivityService(wf.GetMode()).
		SetWorkflow(wf).
		SetWorkflowActivity(wfa)

	resourceOk := modeService.HandleResourceToRunJob(activityID)
	if resourceOk {
		modeService.ApplyJob(activityID)
	}

	config.App().Logger.Infof("WORKER: Activity %d started", activityID)
}
