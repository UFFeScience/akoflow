---
title: Hub and execution APIs
---

The daemon exposes Hub operations below `/akoflow-api/hub/`. Authenticate with the normal daemon credential. The daemon keeps the Hub connection token on the server; callers never supply or receive it. Desktop connections can access only writable publications in the project approved during authorization.

| Method               | Path below `/hub/`                                      | Operation                                                    |
| -------------------- | ------------------------------------------------------- | ------------------------------------------------------------ |
| GET                  | `status/`                                               | Connection and destination status                            |
| GET / POST           | `publications/`                                         | List / create a destination                                  |
| GET / PATCH / DELETE | `publications/{publicationId}/`                         | Read / partially update / soft-delete a publication          |
| GET                  | `publications/{publicationId}/versions/`                | List versions                                                |
| GET                  | `publications/{publicationId}/results/`                 | List published execution results                             |
| GET                  | `publications/{publicationId}/compare/?base=1&target=2` | Compare version YAML                                         |
| POST                 | `publications/workflow-preview/`                        | Review an exported workflow before creating its next version |
| POST                 | `publications/preview/`                                 | Review an execution bundle and selected results              |
| POST                 | `publications/confirm/`                                 | Publish the exact reviewed snapshot                          |
| POST                 | `workflow-definition/`                                  | Fetch an immutable Hub workflow URI                          |
| POST                 | `workflows/import/`                                     | Import that URI as a local workflow                          |

A partial update accepts `title`, `slug`, `description`, `readme`, `visibility`, `authorship`, `tags` and `imageTags`. Omitted fields stay unchanged. Setting public or unlisted visibility also requires `acknowledgePublic: true`. Deletion invalidates pending reviews and clears the default destination when it matches.

To publish a definition without a completed execution, send `workflowId`, `publicationId` and optional `message` to `publications/workflow-preview/`. Send the returned `id` as `previewId` to `publications/confirm/`; public destinations require `acknowledgePublic: true`. Reviews expire after 15 minutes, are bound to the connected account and preserve the reviewed YAML even if the local workflow changes. Version conflicts require a fresh review. Portable exports carry image recipes and sanitized OCI/SIF inspection snapshots.

To import, send `{ "uri": "akoflow://hub/publications/<id>/versions/1", "name": "Imported study" }` to `workflows/import/`. The name is optional. The normal import validator preserves executable source metadata and rejects conflicts rather than replacing existing workflows. Image availability is checked separately during execution preparation.

Execution reads are available below `/akoflow-api/execution-runs/{runId}/` as `status/`, `logs/`, `artifacts/`, `events/?page=1&pageSize=100`, and `activities/{activityId}/`. They use the same persisted views as Desktop. Existing execution creation, preflight, cancel and recovery operations remain in the generated MCP catalog.

The generated catalog contains 175 operations. Hub reads and image inspection are available to the read profile, workflow imports and publication writes require the write profile, and deletion or connection administration require the admin profile. Destructive MCP calls retain their existing review and exact-argument confirmation requirements. JSON recipe registration accepts `dockerfile` or a base64 `contextArchive`, plus `recipePath`, `platform` and `arguments`; multipart uploads remain supported.
