---
title: Find a record
description: Search AkôFlow records by name or ID in Desktop or through the API.
---

# Find a record

Use search to open a workflow, run, environment, plan, or other saved record by name or ID. For operations that finished in this Desktop profile, [follow notifications](./follow-notifications).

## Search from Desktop

1. Focus **Search AkôFlow** in the top bar, press `/`, or press `⌘K`/`Ctrl+K`.
2. Enter an ID, name, state or other indexed value.
3. Select a result, or press Enter to open the first result.
4. Press Escape to close search.

With an empty query, search shows quick navigation destinations. As you type, it shows matching records; select one to open its detail or list page.

Search covers:

| Type | Representative indexed values |
|---|---|
| `workflow` | ID, external ID, name, namespace |
| `execution` | ID, title, resource/runtime IDs, failure and status |
| `artifact` | ID, name and version |
| `environment` | ID, name, description and status |
| `resource` | ID, name, provider, region, zone and type |
| `plan` | ID, algorithm, objective, workflow version and scope |
| `scope` | ID, name, topology and environment versions |
| `materialization` | ID, variant, digest, resource, run, activity, path and status |

Exact matches appear before partial matches.

## Search through the API

Complete [API connection setup](../../tutorials/api-access) first.

```bash
curl --get --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  --data-urlencode 'q=science' \
  --data-urlencode 'types=workflow,execution,artifact' \
  --data-urlencode 'limit=20' \
  "$AKOFLOW_API_URL/search/"
```

The response shape is:

```json
{
  "query": "science",
  "results": [
    {
      "type": "workflow",
      "id": "science-workflow",
      "title": "Science workflow",
      "subtitle": "default",
      "path": "/workflows/science-workflow",
      "score": 0.9
    }
  ],
  "total": 1
}
```

`limit` defaults to 30 and is capped at 100. Omit `types` to search every supported type. Unknown type names are ignored; if every supplied name is unknown, the result is empty. An empty `q` returns an empty result without loading catalogs. A catalog failure returns `500 Internal Server Error`.

## When search returns nothing

- Enter at least one non-whitespace character; an empty server query returns no entities.
- Search uses the same catalogs as the list pages. If a catalog request fails, check the server connection and retry.
- Use an exact ID when possible, or limit API results with `types`.
