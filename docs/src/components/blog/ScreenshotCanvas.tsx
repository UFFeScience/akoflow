import React, { useId } from "react";
import useBaseUrl from "@docusaurus/useBaseUrl";
import styles from "./AnnotatedScreenshot.module.css";

export type ScreenshotAnnotation = {
  from: { x: number; y: number };
  to: { x: number; y: number };
  label: string;
};
export type TutorialShot = {
  src: string;
  title: string;
  caption: string;
  width: number;
  height: number;
  annotations: ScreenshotAnnotation[];
};

export default function ScreenshotCanvas({ shot }: { shot: TutorialShot }) {
  const markerId = useId().replaceAll(":", "");
  const src = useBaseUrl(shot.src);
  return (
    <div className={styles.canvas}>
      <img
        src={src}
        alt={shot.title}
        width={shot.width}
        height={shot.height}
        loading="lazy"
      />
      <svg viewBox={`0 0 ${shot.width} ${shot.height}`} aria-hidden="true">
        <defs>
          <marker
            id={markerId}
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="7"
            markerHeight="7"
            orient="auto-start-reverse"
          >
            <path d="M0 0L10 5L0 10Z" fill="#dc2626" />
          </marker>
        </defs>
        {shot.annotations.map((annotation, index) => (
          <g key={annotation.label}>
            <line
              x1={annotation.from.x}
              y1={annotation.from.y}
              x2={annotation.to.x}
              y2={annotation.to.y}
              stroke="white"
              strokeWidth="7"
            />
            <line
              x1={annotation.from.x}
              y1={annotation.from.y}
              x2={annotation.to.x}
              y2={annotation.to.y}
              stroke="#dc2626"
              strokeWidth="3"
              markerEnd={`url(#${markerId})`}
            />
            <circle
              cx={annotation.from.x}
              cy={annotation.from.y}
              r="17"
              fill="#dc2626"
              stroke="white"
              strokeWidth="3"
            />
            <text
              x={annotation.from.x}
              y={annotation.from.y}
              dominantBaseline="central"
              textAnchor="middle"
              fill="white"
              fontSize="19"
              fontWeight="700"
              fontFamily="system-ui, sans-serif"
            >
              {index + 1}
            </text>
          </g>
        ))}
      </svg>
    </div>
  );
}
