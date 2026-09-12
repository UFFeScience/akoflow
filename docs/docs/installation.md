---
id: installation
title: Install AkôFlow and check the result
sidebar_label: Installation
description: Download the correct Desktop package, open AkôFlow, and verify the installation before connecting HPC or cloud infrastructure.
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

Install **AkôFlow Desktop** on your workstation. It starts a local daemon and
BuildKit using Docker and downloads matching runtime archives automatically.
You do not need a source checkout to install the application.

## Before you begin: prepare Docker

Install Docker before opening AkôFlow. Follow the official instructions for
[macOS](https://docs.docker.com/desktop/setup/install/mac-install/),
[Windows](https://docs.docker.com/desktop/setup/install/windows-install/),
[Ubuntu](https://docs.docker.com/engine/install/ubuntu/), or
[Debian](https://docs.docker.com/engine/install/debian/).
On Linux, include the Compose plugin. On Windows, complete Docker's WSL 2 or
Hyper-V prerequisites and use Linux containers. Start Docker and wait for it
to finish initializing.

Open Terminal on macOS/Linux, or PowerShell on Windows, and run:

```bash
docker info
docker compose version
```

Both commands must succeed **as the user who opens AkôFlow**. The first shows
Docker's server information; the second prints the Compose plugin version.
On Linux, if access works only with `sudo`, follow Docker's
[non-root access instructions](https://docs.docker.com/engine/install/linux-postinstall/#manage-docker-as-a-non-root-user),
then sign out of the desktop session and sign back in before checking again.
Membership in the Docker group grants root-level privileges; use your site's
approved setup. A new terminal alone does not refresh the launcher session.

Keep internet access available for the application and its runtime downloads.
You do not need Git, Go, Node.js, a cloud account, or an HPC account for this
installation. The command-line tutorials introduce their own prerequisites.

## 1. Download the correct file

Open [Downloads and releases](./downloads), choose your operating system, and
save the linked file. Wait until the browser finishes downloading before opening
it. The version in the filename must match the release tag.

| Your computer                 | Package          | Before opening AkôFlow                                                 |
| ----------------------------- | ---------------- | ---------------------------------------------------------------------- |
| macOS, Intel or Apple silicon | Universal `.dmg` | Start Docker Desktop                                                   |
| Windows x64                   | `.exe`           | Start Docker Desktop in Linux-container mode                           |
| Linux x64, Debian/Ubuntu      | `.deb`           | Install Docker Engine and Compose v2; allow your user to access Docker |
| Other Linux x64 desktops      | `.AppImage`      | Same Docker prerequisites; allow the file to execute                   |

The runtime has Linux ARM64 archives, but v1.0.8 does not include a Linux ARM64
Desktop package. Use the [server installation](./guides/operations/server-instance)
for a supported ARM64 server deployment.

**Expected result:** one completed Desktop package for your operating system.
If the browser reports an interrupted download, retry before opening the file.
For checksum verification, use the [download verification procedure](./downloads#download-through-the-github-api).

## 2. Install and open

<Tabs groupId="installation-platform">
<TabItem value="mac" label="macOS">

1. Open the downloaded `.dmg`.
2. Drag **AkôFlow Desktop** into **Applications**.
3. Start Docker Desktop and wait until its engine is running.
4. Open **AkôFlow Desktop** from Applications.

If macOS blocks the application, first confirm the download came from the
[official release](https://github.com/UFFeScience/akoflow/releases/tag/v1.0.8).
See the release notes for signing limitations and your institution's policy
before allowing it to run.

</TabItem>
<TabItem value="windows" label="Windows">

1. Open `Akoflow-Desktop-1.0.8-win-x64.exe` from the browser's download list.
2. Follow the package's installation prompts, if shown, then open AkôFlow Desktop.
3. Keep Docker Desktop running with Linux containers enabled.

The v1.0.8 release lists one Windows executable; it does not offer separately
named installer and portable downloads. Do not look for an additional
`portable.exe` asset in that release.

</TabItem>
<TabItem value="linux" label="Linux">

For Debian/Ubuntu, open a terminal in the directory containing the download:

```bash
sudo apt install ./Akoflow-Desktop-1.0.8-linux-amd64.deb
```

Open **AkôFlow Desktop** from the application launcher. For the AppImage:

```bash
chmod +x Akoflow-Desktop-1.0.8-linux-x86_64.AppImage
./Akoflow-Desktop-1.0.8-linux-x86_64.AppImage
```

</TabItem>
</Tabs>

**Expected result:** AkôFlow opens and displays **Preparing AkôFlow**, followed
by the welcome screen. The initial preparation may take several minutes.
If **AkôFlow could not start** appears, read the error and use
[installation recovery](#recover-at-the-step-that-failed) below.

## 3. First launch: what happens

Desktop checks Docker and Compose, downloads the daemon and BuildKit archives
for its own version and architecture, verifies their SHA-256 checksums, loads
them into Docker, and starts the local services. Keep internet access available
for this first launch. You do not need to download the `.tar` files yourself.

The application's proxy supplies the local API credential. Do not paste a token
into a form just to complete packaged Desktop installation.

### Welcome

Choose **Configure environment** to follow the complete local setup below.
The assistant proceeds through **Welcome → Engine checkup → Environment →
Connection → Ready**.

If you only want to connect HPC or Google Cloud later, **Set up later** opens
the main interface immediately. Continue at [Installation result](#4-installation-result),
then follow the relevant remote tutorial; skipping does not register an environment.

![First-use welcome screen with Configure environment and Set up later](./../static/img/interface/onboarding/welcome.png)

_Welcome screen from the official v1.0.8 Linux application._

### Engine checkup

Wait for **AkôFlow daemon**, **Docker daemon on server**, and **BuildKit service**
to pass, then choose **Continue**. If any check fails, read its message, correct
the dependency and choose **Run again**. Continue stays disabled until all three
checks pass.

![Successful first-launch checkup: daemon, Docker and BuildKit are available](../static/img/interface/onboarding/engine-checkup.png)

_Observed result from the downloaded Linux package: all three services were
available. This check does not test an HPC cluster or a cloud credential._

### Environment: configure the local execution target

Select **Local machine**. Keep **Environment name** as `Local environment`,
or use a unique name, and optionally edit **Description**. Choose
**Configure and check**.

![Environment step with Local machine selected and Configure and check visible](../static/img/interface/onboarding/environment-selection.png)

_Local machine configures execution through the daemon. With packaged Desktop,
that daemon runs in Docker; this is not a measurement of your laptop's full
compute capacity. Review discovered resources before planning real work._

The initial assistant also offers **Supercomputer / HPC** and **Kubernetes cluster**.
For HPC, use the [guided registration tutorial](./tutorials/register-hpc) to
prepare and authorize an SSH key before connecting. For Kubernetes, use the
[cluster guide](./guides/infrastructure/kubernetes). Google Cloud is connected
from the main **Environments** catalog after leaving this assistant.

### Connection: wait for registration, health and discovery

The application registers the environment, checks its connection, and discovers
resources. Let these operations finish; success advances to **Ready** automatically.

![Connection step while the application checks the local environment](../static/img/interface/onboarding/connection-checkup.png)

_This step can be brief. The next screen is the completion checkpoint._

If **Checkup needs attention** appears, read the failing operation. Use
**Edit connection** to correct the settings or **Run checkup again** to retry.
Registration can finish before a later check fails, so inspect the existing
entry rather than creating another environment with a different name.

### Ready: open the application

Wait for **Local environment is ready**, then choose **Open AkôFlow**.
**Add another environment** returns to the environment step when you want to
configure another target.

![Ready step confirming the local environment and offering Open AkôFlow](../static/img/interface/onboarding/environment-ready.png)

Open **Infrastructure → Environments** and confirm that **Local environment**
appears. Open it to inspect its connection and inventory.

![Environment catalog after completing local setup](../static/img/interface/onboarding/environment-catalog.png)

**Expected result:** the local environment is saved, its connection check has
passed, and discovery has completed. No workflow has run yet. The full local
assistant was exercised with the official Linux package; remote success requires
the checks in the relevant infrastructure tutorial.

## 4. Installation result

### Check through the interface

| Checkpoint           | Expected result                                      | If it fails                                                                        |
| -------------------- | ---------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Application opens    | Welcome screen or Overview appears                   | Read startup error details and confirm Docker/Compose access                       |
| Local control plane  | Sidebar connection indicator says **Connected**      | Wait for startup; inspect the daemon failure rather than creating a new identity   |
| Catalog navigation   | **Infrastructure → Environments** opens              | Use [Troubleshooting](./guides/operations/troubleshooting) for API/instance errors |
| Infrastructure setup | **Connect environment** opens the connection choices | Continue with the HPC or cloud tutorial below                                      |

![Overview after first launch with the local control-plane connection established](../static/img/interface/onboarding/installation-result.png)

_Overview after choosing Set up later on a fresh installation. If you completed
the local assistant, your environment is already present; empty execution charts
are still expected before running workflows._

An empty catalog is normal on a new installation. A connected daemon confirms
that the application can reach its control plane; it does not prove that a
remote cluster, a cloud credential, or a workflow is ready.

### Check through the API

For a daemon you manage directly, first follow [API connection setup](./tutorials/api-access).
Run these public bootstrap checks against **that server**, using its actual address:

```bash
curl --fail-with-body "$AKOFLOW_API_URL/preflight/" | jq
curl --fail-with-body "$AKOFLOW_API_URL/instance/" | jq '{id, name}'
```

Expect `server.available: true` and a non-empty instance ID. Review `docker` and
`buildkit` separately: server availability alone does not confirm either build
prerequisite. These checks diagnose an existing installation; HTTP requests do
not install the Desktop package. The packaged application's internal port and
credential are managed by Desktop; do not assume they are `8080` and a token
from a development checkout.

## 5. Continue with your execution target

- [Run your first local workflow in Desktop](./guides/workflows/first-local-run): create one activity, run it, and inspect its generated file.
- [Register an HPC / SLURM environment](./tutorials/register-hpc): authorize an SSH key, test the login connection, and discover partitions.
- [Connect Google Cloud](./tutorials/connect-cloud): validate a service account, register the environment, and synchronize the compute catalog.
- [Run the first simulated workflow](./guides/workflows/first-run): verify workflow execution through the documented API setup without a remote account.

## Recover at the step that failed

| Where you stopped                          | What to do                                                                                                                |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| Docker command is missing                  | Complete the official Docker installation above, reopen the terminal and rerun both checks.                               |
| Docker reports permission denied on Linux  | Complete Docker access setup, sign out and back in, then reopen AkôFlow as your normal user.                              |
| Compose command is missing                 | Install the Compose plugin for your Docker installation; `docker-compose` alone is not the documented command.            |
| Preparing AkôFlow fails during download    | Read the displayed error, restore network access to GitHub release assets, then quit and reopen the application to retry. |
| AkôFlow could not start                    | Verify both Docker commands, read the startup error, correct the reported cause and reopen the app.                       |
| Engine checkup has a failed service        | Read the service message; restore Docker/BuildKit availability and choose Run again.                                      |
| Connection checkup needs attention         | Use Edit connection or Run checkup again; an environment may already have been registered.                                |
| Welcome no longer appears                  | This is expected after completing or skipping setup. Continue from Infrastructure → Environments.                         |
| The app is connected but there are no runs | Continue with a workflow tutorial; installation does not submit a workflow.                                               |

For further diagnosis, use [Troubleshooting](./guides/operations/troubleshooting).
Keep the failed step, exact error, operating system and Desktop version when
requesting help.

## Updates and verification scope

Export your instance before changing versions; see [Instance management](./guides/operations/instance-management).
Keep Desktop and its runtime on matching versions.

The v1.0.8 Linux `.deb` was fully downloaded on 2026-09-12. Its SHA-256 matches
the GitHub asset digest, and its package metadata reports version `1.0.8`,
architecture `amd64`. The application extracted from that package also started its release-matched
daemon and BuildKit in Docker and reached the successful checkup shown above.
This was an extracted-package smoke test on Linux with a fresh application
profile, not a test of the `apt` installation procedure or of macOS/Windows.
