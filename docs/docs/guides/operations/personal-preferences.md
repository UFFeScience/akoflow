---
title: Set personal preferences
description: Change the Desktop theme and graph animation for one browser profile.
---

# Set personal preferences

Use this guide to change AkôFlow Desktop's theme and graph animation. For the API commands below, complete [API connection setup](../../tutorials/api-access) first.

Preferences belong to a browser-profile client ID rather than a user account. Desktop saves changes locally and tries to synchronize them with the server. Your choices remain available in that browser when the server is offline.

## Using AkôFlow Desktop

1. Open **Settings → General**.
2. Select **Light** or **Dark**.
3. Turn **Graph animation** on or off.

## Using the API

The client ID must contain 8–128 characters. The only accepted themes are `light` and `dark`.

```bash
CLIENT_ID='docs-client-01'

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -X PUT "$AKOFLOW_API_URL/user-preferences/$CLIENT_ID/" \
  -d '{"theme":"dark","animationsEnabled":false}'

curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/user-preferences/$CLIENT_ID/"
```
