import type { TutorialShot } from "./ScreenshotCanvas";

const tutorialShots = {
  connected: {
    src: "/img/blog/first-local-run/desktop-connected.jpg",
    title: "Begin on Overview after installation",
    caption:
      "The capture shows an existing instance. Your fresh installation can have empty catalogs.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 210,
          y: 590,
        },
        to: {
          x: 90,
          y: 646,
        },
        label: "Confirm that the bottom-left indicator says Connected.",
      },
      {
        from: {
          x: 200,
          y: 235,
        },
        to: {
          x: 115,
          y: 292,
        },
        label:
          "Open Infrastructure \u2192 Environments to create your local target.",
      },
    ],
  },
  environments: {
    src: "/img/blog/first-local-run/connect-environment.jpg",
    title: "Open the local environment setup",
    caption:
      "Use the execution-infrastructure catalog for this real local workflow.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 860,
          y: 110,
        },
        to: {
          x: 991,
          y: 170,
        },
        label:
          "Select Connect environment. Some releases call this New environment.",
      },
      {
        from: {
          x: 580,
          y: 280,
        },
        to: {
          x: 397,
          y: 234,
        },
        label: "Keep Execution infrastructure selected.",
      },
    ],
  },
  localForm: {
    src: "/img/blog/first-local-run/new-local-environment.jpg",
    title: "Create Local check",
    caption:
      "This form has only been filled for the capture: Save environment becomes available after a successful connection test.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 530,
          y: 215,
        },
        to: {
          x: 420,
          y: 289,
        },
        label: "Choose Local machine.",
      },
      {
        from: {
          x: 610,
          y: 421,
        },
        to: {
          x: 527,
          y: 485,
        },
        label: "Enter Local check as the environment name.",
      },
      {
        from: {
          x: 812,
          y: 605,
        },
        to: {
          x: 952,
          y: 635,
        },
        label: "Select Test connection and wait for success.",
      },
      {
        from: {
          x: 1130,
          y: 575,
        },
        to: {
          x: 1121,
          y: 635,
        },
        label: "After the test succeeds, select Save environment.",
      },
    ],
  },
  import: {
    src: "/img/blog/first-local-run/import-workflow.jpg",
    title: "Import the workflow you saved",
    caption:
      "The catalog can already contain other workflows. Import your first-local-check.yaml file.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 1030,
          y: 160,
        },
        to: {
          x: 1162,
          y: 110,
        },
        label: "Select Import YAML, then choose first-local-check.yaml.",
      },
      {
        from: {
          x: 190,
          y: 205,
        },
        to: {
          x: 111,
          y: 178,
        },
        label: "Find this catalog under Workflows \u2192 Definitions.",
      },
    ],
  },
  scopeCatalog: {
    src: "/img/blog/first-local-run/scope-catalog.jpg",
    title: "Create a new execution scope",
    caption:
      "A fresh instance can show an empty catalog; the creation action is in the page header.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 1050,
          y: 245,
        },
        to: {
          x: 1165,
          y: 131,
        },
        label: "Select Create scope (New execution scope in some releases).",
      },
      {
        from: {
          x: 189,
          y: 446,
        },
        to: {
          x: 121,
          y: 405,
        },
        label: "Open Infrastructure \u2192 Execution scopes.",
      },
    ],
  },
  scopeName: {
    src: "/img/blog/first-local-run/scope-name.jpg",
    title: "Name the scope and keep its topology",
    caption:
      "Enter First local scope. The initial topology is sufficient for a one-machine experiment.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 580,
          y: 260,
        },
        to: {
          x: 561,
          y: 319,
        },
        label: "Enter First local scope in Name.",
      },
      {
        from: {
          x: 531,
          y: 415,
        },
        to: {
          x: 325,
          y: 478,
        },
        label: "Leave Create the initial network topology enabled.",
      },
    ],
  },
  scopeEnvironment: {
    src: "/img/blog/first-local-run/new-local-scope.jpg",
    title: "Select only the local environment for this experiment",
    caption:
      "The reference instance calls its existing environment Local environment. In your tutorial, choose Local check.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 580,
          y: 175,
        },
        to: {
          x: 325,
          y: 208,
        },
        label: "Keep the initial topology enabled.",
      },
      {
        from: {
          x: 1060,
          y: 327,
        },
        to: {
          x: 780,
          y: 392,
        },
        label:
          "Select the checkbox for your local environment; use Local check from step 2.",
      },
    ],
  },
  scopeSave: {
    src: "/img/blog/first-local-run/save-scope.jpg",
    title: "Save the execution scope",
    caption:
      "Scroll below the list of participating environments to reach this action.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 1040,
          y: 570,
        },
        to: {
          x: 1137,
          y: 635,
        },
        label:
          "Select Create execution scope after naming the scope and selecting Local check.",
      },
    ],
  },
  planAction: {
    src: "/img/blog/first-local-run/workflow-plan-action.jpg",
    title: "Start planning from the workflow",
    caption:
      "This reference definition is named hello. Open first-local-check in your own catalog.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 1010,
          y: 275,
        },
        to: {
          x: 1160,
          y: 215,
        },
        label: "Select Generate plan on the workflow detail page.",
      },
    ],
  },
  manualMode: {
    src: "/img/blog/first-local-run/manual-plan-mode.jpg",
    title: "Choose manual planning",
    caption:
      "The reference definition has two activities. Your first-local-check definition has only write-report.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 1025,
          y: 160,
        },
        to: {
          x: 995,
          y: 234,
        },
        label: "Select Create manually.",
      },
      {
        from: {
          x: 930,
          y: 506,
        },
        to: {
          x: 1040,
          y: 558,
        },
        label: "Use Real execution.",
      },
      {
        from: {
          x: 565,
          y: 570,
        },
        to: {
          x: 486,
          y: 609,
        },
        label:
          "Under Execution scope, choose First local scope; scroll the list if needed.",
      },
    ],
  },
  placement: {
    src: "/img/blog/first-local-run/manual-activity-placement.jpg",
    title: "Assign a runtime, duration, and target",
    caption:
      "This reference activity is named a. In your workflow, select write-report; configure the same controls.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 355,
          y: 230,
        },
        to: {
          x: 575,
          y: 293,
        },
        label: "Select the activity node in the DAG.",
      },
      {
        from: {
          x: 1085,
          y: 154,
        },
        to: {
          x: 970,
          y: 203,
        },
        label: "Choose the local machine runtime.",
      },
      {
        from: {
          x: 850,
          y: 276,
        },
        to: {
          x: 979,
          y: 307,
        },
        label: "Set Expected execution time to 1 and keep the s unit.",
      },
      {
        from: {
          x: 1115,
          y: 370,
        },
        to: {
          x: 984,
          y: 415,
        },
        label:
          "Choose your local machine target. Then scroll to the plan review and Generate plan.",
      },
    ],
  },
  execute: {
    src: "/img/blog/first-local-run/execute-plan.jpg",
    title: "Open the execution request from the saved plan",
    caption:
      "The example capture shows an existing saved plan. Use the manual plan generated for first-local-check.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 1022,
          y: 250,
        },
        to: {
          x: 1165,
          y: 196,
        },
        label: "Select Execute plan.",
      },
    ],
  },
  review: {
    src: "/img/blog/first-local-run/start-real-execution.jpg",
    title: "Review the execution before starting",
    caption:
      "This reference plan is bound to its execution request. Your scope and plan names should refer to your new tutorial records.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 478,
          y: 252,
        },
        to: {
          x: 505,
          y: 319,
        },
        label: "Keep the suggested run ID or enter a fresh unique ID.",
      },
      {
        from: {
          x: 1060,
          y: 252,
        },
        to: {
          x: 972,
          y: 320,
        },
        label: "Confirm that the selected plan is the one you just generated.",
      },
      {
        from: {
          x: 798,
          y: 440,
        },
        to: {
          x: 641,
          y: 516,
        },
        label: "Confirm Real execution and an available local runtime.",
      },
    ],
  },
  start: {
    src: "/img/blog/first-local-run/start-execution-button.jpg",
    title: "Start the local run",
    caption:
      "Scroll to the bottom of the execution request after reviewing the selected plan.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 996,
          y: 602,
        },
        to: {
          x: 1160,
          y: 635,
        },
        label:
          "Select Start execution once. Submission is asynchronous; wait on the resulting run page.",
      },
    ],
  },
  completed: {
    src: "/img/blog/first-local-run/completed-run.jpg",
    title: "Check completion and open Activities",
    caption:
      "This real reference run has two activities. Your first-local-check run must show 1/1 settled.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 409,
          y: 220,
        },
        to: {
          x: 342,
          y: 291,
        },
        label: "Wait for completed.",
      },
      {
        from: {
          x: 649,
          y: 219,
        },
        to: {
          x: 577,
          y: 291,
        },
        label: "Check 1/1 settled for your one-activity workflow.",
      },
      {
        from: {
          x: 473,
          y: 425,
        },
        to: {
          x: 405,
          y: 382,
        },
        label: "Open Activities to locate write-report.",
      },
    ],
  },
  activity: {
    src: "/img/blog/first-local-run/open-activity-details.jpg",
    title: "Locate the activity to inspect its output",
    caption:
      "This reference run lists a and b. Your activity list contains write-report.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 893,
          y: 486,
        },
        to: {
          x: 1096,
          y: 580,
        },
        label:
          "Select write-report in your activity list. Use Details or Open activity page in its information panel.",
      },
    ],
  },
  files: {
    src: "/img/blog/first-local-run/generated-files.jpg",
    title: "Find the output and its checksum",
    caption:
      "The reference run produced a.txt. Your YAML produces result.txt and prints Akoflow local check.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: {
          x: 1110,
          y: 300,
        },
        to: {
          x: 748,
          y: 402,
        },
        label: "Inspect Activity output for the captured runtime log.",
      },
      {
        from: {
          x: 544,
          y: 524,
        },
        to: {
          x: 485,
          y: 620,
        },
        label: "In Generated files, look for result.txt in your run.",
      },
      {
        from: {
          x: 945,
          y: 568,
        },
        to: {
          x: 872,
          y: 620,
        },
        label: "Confirm that the file has a SHA-256 checksum.",
      },
    ],
  },
  checkOnline: {
    src: "/img/blog/first-local-run/check-local-online.jpg",
    title: "Check that the local machine is online",
    caption:
      "Open the environment you created. This reference environment is already online; your tutorial environment is named Local check.",
    width: 1280,
    height: 720,
    annotations: [
      {
        from: { x: 735, y: 440 },
        to: { x: 617, y: 391 },
        label: "Select Check now to request a fresh connection check.",
      },
      {
        from: { x: 1030, y: 318 },
        to: { x: 532, y: 391 },
        label: "Continue when the local connection reads online.",
      },
    ],
  },
} satisfies Record<string, TutorialShot>;

export default tutorialShots;
