---
title: Manage SSH service keys
description: Generate or import an SSH key and assign it to a saved connection.
---

# Manage SSH service keys

Use this guide when an SSH connection needs a key managed by AkôFlow. Generate or import the key, authorize its public half on the remote host, then assign its reference to the saved connection. Keep the private key out of workflow definitions.

For other providers, use [Configure Kubernetes](../infrastructure/kubernetes) to store a cluster token or [Connect Google Cloud](../../tutorials/connect-cloud) to validate and store a service-account credential.

AkôFlow returns public metadata and a credential reference when you list managed SSH keys; it does not return the original private key.

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

Read the saved connection from its environment and change only `credentialRef`.
Use the environment ID, connection ID, and reference returned by key registration:

```bash
read -r -p 'Environment ID: ' AKOFLOW_ENVIRONMENT_ID || exit 1
read -r -p 'Connection ID: ' AKOFLOW_CONNECTION_ID || exit 1
read -r -p 'New credentialRef: ' AKOFLOW_CREDENTIAL_REF || exit 1
[ -n "$AKOFLOW_ENVIRONMENT_ID" ] && [ -n "$AKOFLOW_CONNECTION_ID" ] &&
  [ -n "$AKOFLOW_CREDENTIAL_REF" ] || exit 1

(
  set -o pipefail
  curl --fail-with-body \
    -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    "$AKOFLOW_API_URL/environments/$AKOFLOW_ENVIRONMENT_ID/" |
    jq -e --arg id "$AKOFLOW_CONNECTION_ID" \
      --arg ref "$AKOFLOW_CREDENTIAL_REF" \
      '.connections[] | select(.id == $id) | .credentialRef = $ref' |
    curl --fail-with-body -X PUT \
      -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
      -H 'Content-Type: application/json' \
      --data-binary @- \
      "$AKOFLOW_API_URL/environment-connections/$AKOFLOW_CONNECTION_ID/"
) || exit 1

curl --fail-with-body -X POST \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environment-connections/$AKOFLOW_CONNECTION_ID/health/"
```

The `PUT` replaces stored fields; the separate health request tests the updated
connection. Inspect its returned status before using it. A missing connection ID
makes `jq` fail; correct the ID rather than creating a second connection.

## Keep the key private

- Do not place private keys in workflow YAML, resource metadata, screenshots, logs, or documentation examples.
- API Bearer authentication protects requests to the server; `credentialRef` identifies the saved credential used for a provider operation.
- Instance export redacts credentials and credential references. Imported snapshots therefore cannot reconnect until you return to a writable instance and configure credentials there.
- If SSH uses a gateway or proxy command, authorize and validate every hop. `forwardAgent` and proxy settings are connection configuration, not substitutes for a server-managed key.
- A leaked public key does not reveal the private key, but remote `authorized_keys` entries still determine where that key can authenticate.
