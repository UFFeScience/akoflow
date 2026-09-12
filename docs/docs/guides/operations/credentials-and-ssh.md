---
title: Manage credentials and SSH service keys
description: Store credentials with AkôFlow, assign SSH service keys, and keep private material outside workflow definitions.
---

# Manage credentials and SSH service keys

Use this guide when an environment needs an SSH key, Kubernetes token, or cloud credential. Store the credential in AkôFlow and use the returned reference in the connection that needs it. Keep private keys and tokens out of workflow definitions.

AkôFlow returns public metadata or a credential reference when you list stored credentials; it does not return the original private key or bearer token.

## Generate an SSH service key

### Using AkôFlow Desktop

1. Open **Settings → SSH service keys**.
2. Under **Register a service key**, enter a key ID and optional public-key comment.
3. Select **Generate key**.
4. Copy the public key and authorize it on every SSH hop required by the target—for example, both a gateway and its HPC login node.

The AkôFlow server generates an Ed25519 key. IDs must start with an ASCII letter or digit, may then contain letters, digits, `_` or `-`, and may contain at most 64 characters. The private file is stored with mode `0600`.

### Using the API

Complete [API connection setup](../../tutorials/api-access) first.

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_API_URL/ssh-keys/" \
  -d '{"id":"plafrim-service","comment":"akoflow@plafrim"}'
```

The response contains `id`, `credentialRef`, `publicKey`, and SHA-256 `fingerprint`. A duplicate or invalid ID returns `422`; missing key-management support returns `503`.

## Import an existing OpenSSH private key

### Using AkôFlow Desktop

1. Open **Settings → SSH service keys**.
2. Enter a new **Key ID** under **Import an existing private key**.
3. Paste the OpenSSH private key and select **Import private key**.

The private key is sent once to the AkôFlow server, validated with `ssh-keygen`, stored in the credential directory and never displayed again.

### Using the API

Avoid putting a private key in shell history or a temporary JSON file. Read the existing protected key file and stream the request:

```bash
(
  set -o pipefail
  jq -n \
    --arg id 'existing-hpc-key' \
    --rawfile privateKey "$HOME/.ssh/id_ed25519" \
    '{id:$id, privateKey:$privateKey}' | curl --fail-with-body \
    -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/json' \
    -X POST "$AKOFLOW_API_URL/ssh-keys/import/" \
    --data-binary @-
)
```

An empty or invalid private key, invalid/duplicate ID, or `ssh-keygen` failure returns `422`.

List public metadata at any time:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/ssh-keys/"
```

There is currently no SSH-key deletion endpoint. When a key should no longer be trusted, remove its authorization on remote systems and follow your operator's key-rotation procedure.

## Assign a key to a connection

Generating a key does not grant access. A connection must reference it, and the public key must be authorized remotely.

### Using AkôFlow Desktop

1. On a managed key card, choose **Assign to SSH connection**.
2. Select the environment and connection.
3. Select **Assign / update key**.
4. Run the connection health check from the environment.

Assignment changes only `credentialRef`; it preserves the connection's endpoint, user and configuration.

### Using the API

Read the current environment definition first so you preserve every connection field. Then update the connection with the `credentialRef` returned above:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -X PUT "$AKOFLOW_API_URL/environment-connections/hpc-ssh/" \
  -d '{
    "id":"hpc-ssh",
    "environmentId":"plafrim",
    "name":"PlaFRIM login",
    "type":"ssh",
    "endpoint":"plafrim.example.org:22",
    "username":"researcher",
    "credentialRef":"<credentialRef returned by /ssh-keys/>"
  }'
```

## Kubernetes bearer tokens

The Desktop environment connection flow stores a Kubernetes token and retains only its reference. For direct API use, put the token in a file readable only by your user and set `KUBE_TOKEN_FILE` to its path. The command reads that file without placing the token in shell history:

```bash
KUBE_TOKEN_FILE="$HOME/.kube/akoflow-token"

(
  set -o pipefail
  jq -n --arg id 'research-cluster' --rawfile token "$KUBE_TOKEN_FILE" \
    '{id:$id, token:$token}' | curl --fail-with-body \
    -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/json' \
    -X POST "$AKOFLOW_API_URL/kubernetes-tokens/" \
    --data-binary @-
)
```

The response is `{"credentialRef":"..."}`. Empty/invalid values return `422`; unavailable credential storage returns `503`.

## Cloud credentials

Cloud onboarding similarly sends provider credential JSON to `/cloud-credentials/` and stores only the returned reference. Validation is a separate operation at `/cloud-credentials/validate/`; see [Cloud capacity](../infrastructure/cloud-capacity.md) for provider-specific fields and the complete flow.

## Security boundaries

- Do not place private keys or tokens in workflow YAML, resource metadata, screenshots, logs, or documentation examples.
- API Bearer authentication protects requests to the server; `credentialRef` identifies the saved credential used for a provider operation.
- Instance export redacts credentials and credential references. Imported snapshots therefore cannot reconnect until you return to a writable instance and configure credentials there.
- If SSH uses a gateway or proxy command, authorize and validate every hop. `forwardAgent` and proxy settings are connection configuration, not substitutes for a server-managed key.
- A leaked public key does not reveal the private key, but remote `authorized_keys` entries still determine where that key can authenticate.
