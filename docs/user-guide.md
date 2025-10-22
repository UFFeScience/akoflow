---
title: 3 - User Guide
description: This guide provides an overview of how to use AkôFlow, including creating workflows, managing resources, and monitoring execution.
layout: page
---

## Getting Started with AkôFlow
Welcome to the AkôFlow User Guide! This document will help you get started with using AkôFlow to create and manage your workflows effectively.

### Accessing the AkôFlow Interface
Once you have installed AkôFlow, you can access the web interface by navigating to `http://localhost:8080` in your web browser.

### Creating Your First Workflow

1. **Create a New Workflow:**
   - Click on the "New Workflow" button on the dashboard.

1.1 **You can Use .YAML or the Visual Editor:**
   - **YAML Editor:** If you prefer to define your workflow using YAML, select the YAML editor option. Here is a simple example of a workflow definition:
     ```yaml
        name: wf-hello-world
            spec:
            image: "alpine:latest"
            namespace: "akoflow"
            storageClassName: "hostpath"
            storageSize: "32Mi"
            storagePolicy:
                type: distributed
            mountPath: "/data"
            activities:
                - name: "a"
                memoryLimit: 500Mi
                cpuLimit: 0.5
                run: |
                    ls -la
                    echo "Hello from a"
                    echo "Hello from a" > a.txt

                - name: "b"
                memoryLimit: 500Mi
                cpuLimit: 0.5
                run: |
                    ls -la
                    echo "Hello from b"
                    echo "Hello from b" > b.txt
     ```
   - **Visual Editor:** Alternatively, you can use the visual editor to drag and drop components to build your workflow.

2. **Define Workflow Steps:**
    - Add steps to your workflow by specifying the tasks you want to execute. You can define each step's parameters, dependencies, and execution conditions.

3. **Save and Execute:**
    - Once you have defined your workflow, click "Save" to store it. You can

### Workflow Specifications
AkôFlow workflows are defined using YAML files. Below is a breakdown of the key components of a workflow specification:

Refer to the [Workflow Specification Documentation](internal/workflow-spec.md) for detailed information on each field.


### API 

AkôFlow provides a RESTful API that allows you to programmatically manage workflows. You can use the API to create, update, delete, and monitor workflows.

Refer to the [API Documentation](internal/api.md) for detailed information on available endpoints and usage examples.

### Monitoring and Logs