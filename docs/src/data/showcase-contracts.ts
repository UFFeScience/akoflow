import data from "./showcase-contracts.json";

export type ShowcaseContract = {
  title: string;
  examplePath: string;
  stages: string[][];
  dependencies: Record<string, string[]>;
  validationMode: string;
  validationStatus?: string;
  localProfile?: string;
  sourceCommit?: string;
  requirements?: string[];
  artifacts?: string[];
  executionMode?: string;
  screenshots?: Record<string, string>;
  daemon?: {
    runId: string;
    workflowId: string;
    planId: string;
    imageId: string;
    status: string;
    activityCount: number;
    completedActivityCount?: number;
  };
};

export const showcaseContracts = data as Record<string, ShowcaseContract>;

export function findShowcase(value: string): ShowcaseContract | undefined {
  return Object.values(showcaseContracts).find(
    (item) => item.title === value || item.examplePath === value,
  );
}
