---
title: Run your first workflow in Desktop
description: Create a one-activity local workflow, run it, and inspect its output file.
---

# Run your first workflow in Desktop

This tutorial runs one activity on your local machine. You will create a workflow, choose where it runs, start it, and inspect the recorded output file and checksum. It uses the Desktop interface and does not require a cloud or HPC account.

## Before you begin

[Install AkôFlow Desktop](/docs/installation) and complete its first-start checkup. Docker must be available to your user, and Desktop must show **Connected**. The steps below were verified with the extracted v1.0.8 Linux Desktop package and its bundled runtime. A clean `apt` install and other platforms need their own check.

## 1. Check your local environment

Open **Infrastructure → Environments**. If the first-start checkup already created a local environment, open it. Otherwise, choose **New environment**, select **Local machine**, give it a name such as `Local check`, select **Test connection**, then **Save environment**.

On the environment page, select **Check now**. Continue when **Local machine** shows **online**. This check also makes the local resource available for planning.

## 2. Define a small workflow

Open **Workflows → New workflow** and enter:

| Field | Value |
| --- | --- |
| Workflow name | `First local check` |
| Activity name | `write-report` |
| Command | `printf 'Akoflow local check\n' > result.txt` |

Leave the other activity settings at their defaults and select **Create workflow**. The workflow page should show one activity, `write-report`.

## 3. Choose the local environment

Open **Infrastructure → Execution scopes** and choose **New execution scope**. Name it `First local scope`, select your local environment, leave **Create the initial network topology** enabled, and select **Create execution scope**. A one-machine workflow needs no network links.

## 4. Make and run a plan

Return to the workflow and select **Generate plan**. Choose **Create manually**, then select `First local scope`. Select `write-report` in the activity graph and set:

| Setting | Value |
| --- | --- |
| Runtime | Your local machine runtime |
| Expected execution time | `1` second |
| Machine / target | Your local machine resource |

Select **Generate plan**. On the plan page, select **Execute plan**. Confirm that the execution mode is **Real execution** and the local runtime is listed, then select **Start execution**.

## 5. Check the result

Wait for the run to show **completed** and **1/1** activities settled. Open **Activities → write-report → Open activity details**. Expect **Exit code 0** and a **Generated files** row for `result.txt` with a SHA-256 checksum. That row records the file observed in the activity workspace; it does not mean Desktop downloaded the file to your computer.

If the run fails, inspect the activity output on that page and use [Troubleshooting](/docs/guides/operations/troubleshooting). To understand how the plan and run records relate, read [Compare a plan with a completed run](/docs/explanations/evidence-and-provenance). To try scheduling alternatives or another target, continue with [Plan a workflow](/docs/guides/workflows/planning) and [Environments](/docs/guides/infrastructure/environments). For a reproducible simulation through a separately managed API, use the [SimGrid example](/docs/guides/workflows/first-run).
