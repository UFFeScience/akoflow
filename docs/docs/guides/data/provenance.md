---
title: Trace a result with provenance
description: Use the read-only SQL workspace to connect a workflow, plan, run, activities, transfers, and data.
---

# Trace a result with provenance

Open **Provenance** in AkôFlow Desktop to query the local evidence database. The current interface provides a read-only SQL workspace. It does not have separate Explore or Lineage views.

## Query with read-only SQL

1. Open the completed run and copy its run ID.
2. In **Provenance**, select a table in **Safe schema** to create a starter `SELECT` query.
3. Filter by the run ID and join related tables when the question spans plans, activities, transfers, or data.
4. Run the query and export the result page with its SQL and parameters.

For the annotated screen, API calls, pagination, and an investigation example, see [Provenance and audit](/docs/guides/data/provenance-and-audit).
