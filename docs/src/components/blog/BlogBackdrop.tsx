import React from "react";
import styles from "./blog.module.css";

/** Decorative network: it is not a workflow definition or execution graph. */
export default function BlogBackdrop() {
  return (
    <div className={styles.backdrop} aria-hidden="true">
      <svg viewBox="0 0 1440 900" preserveAspectRatio="xMidYMid slice">
        <path d="M-80 220H180Q240 220 240 280V420H480Q540 420 540 360V180H820Q880 180 880 240V420H1120Q1180 420 1180 360V140H1520M240 420V650H540Q600 650 600 710V810H900Q960 810 960 750V600H1240Q1300 600 1300 660V940M540 420H820Q880 420 880 480V600H1240M-60 750H180Q240 750 240 690V650" />
        {[
          [180, 220],
          [240, 420],
          [480, 420],
          [540, 180],
          [820, 180],
          [880, 420],
          [1120, 420],
          [1180, 140],
          [240, 650],
          [540, 650],
          [600, 810],
          [900, 810],
          [960, 600],
          [1240, 600],
        ].map(([x, y]) => (
          <g key={`${x}-${y}`}>
            <circle cx={x} cy={y} r="13" />
            <circle cx={x} cy={y} r="4" />
          </g>
        ))}
      </svg>
    </div>
  );
}
