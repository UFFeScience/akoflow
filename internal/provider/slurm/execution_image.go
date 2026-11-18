package slurm

import "github.com/UFFeScience/akoflow/internal/domain"

func prepareImageObservation(handle *domain.ActivityHandle, command domain.ActivityCommand) {
	image, _ := slurmExecutable(command)
	if image == "" {
		return
	}
	requested := command.Image
	if source := command.EffectiveExecutable(); source != nil && source.Source.Reference != "" {
		requested = source.Source.Reference
	}
	observation := map[string]any{"status": "unavailable", "requestedReference": requested}
	if resolved := command.ResolvedExecutable; resolved != nil && resolved.MaterializationDone && resolved.VariantID != "" && resolved.Digest != "" {
		observation["sifVariantId"] = resolved.VariantID
		observation["sifDigest"] = resolved.Digest
	}
	handle.Metadata["imageObservation"] = observation
}

func confirmImageObservation(handle *domain.ActivityHandle) {
	observation, _ := handle.Metadata["imageObservation"].(map[string]any)
	if observation["sifVariantId"] != nil {
		observation["status"] = "observed"
	}
}
