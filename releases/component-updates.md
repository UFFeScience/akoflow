# Desktop component releases

A full installer establishes the native compatibility version and embeds the public Ed25519 trust anchor. The release job signs manifests with the repository secret `AKOFLOW_COMPONENT_SIGNING_KEY`. Keep the corresponding private key in the maintainer's credential store. Rotate the trust anchor through a full Desktop release.

For a component-only release, create the corresponding frozen version tags and dispatch **Publish GitHub release** with `component_update=true`, the installed Desktop tag in `minimum_desktop`, and `rollback_safe=true`. Set any unchanged component's version to its existing release tag. Frontend and daemon default to the new tag. BuildKit defaults to the installed Desktop tag. The workflow publishes a prerelease and advances the signed component channel after the referenced assets are available.

Verify that the current daemon can open and use data after the replacement daemon has opened it. Use a full release for an irreversible migration. Before that release, create and verify a backup of the runtime volumes and publish a recovery plan that names the compatible daemon, identifies which volumes to restore, and describes how to confirm that history and settings recovered.

Verify the signed update on an installed copy of each supported platform. Check the downloaded asset names for frontend-only, daemon-only, and coordinated updates. Try again after an interrupted transfer, try while an execution is active, and use a deliberately unhealthy test image to confirm that the previous daemon and frontend remain usable. Keep the test channel separate from the production component channel.
